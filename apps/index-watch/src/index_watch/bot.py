"""Telegram bot handlers and scheduled jobs."""

import asyncio
import logging
from datetime import UTC, datetime
from typing import Any

from apscheduler.events import EVENT_JOB_ERROR, EVENT_JOB_EXECUTED
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from telegram import Update
from telegram.ext import Application, CommandHandler, ContextTypes

from index_watch import database
from index_watch.alerts import AlertState
from index_watch.cache import get_cache
from index_watch.config import Config
from index_watch.fear_greed import fetch_fear_greed
from index_watch.formatting import (
    format_daily_report,
    format_drawdown_alert,
    format_drawdown_block,
    format_fear_greed,
    format_historical_frequency,
)
from index_watch.index_data import (
    fetch_index_history,
    get_index_metrics,
    historical_drawdown_frequency,
)
from index_watch.rate_limiter import RATE_LIMITS, RateLimiter

logger = logging.getLogger(__name__)

alert_state = AlertState()
rate_limiter = RateLimiter()


def _build_daily_report(config: Config) -> str:
    """Build the full daily report text (sync, for use from async)."""
    index_blocks: list[tuple[str, str]] = []
    history_blocks: list[str] = []
    data_timestamps: list[datetime] = []
    has_stale_data = False

    for symbol, name in config.index_symbols.items():
        result = get_index_metrics(symbol, name, years=config.history_years)
        if result:
            metrics, fetched_at, is_stale = result
            data_timestamps.append(fetched_at)
            if is_stale:
                has_stale_data = True
            index_blocks.append((name, format_drawdown_block(name, metrics)))
            closes, _, _ = fetch_index_history(symbol, years=config.history_years)
            if closes:
                freq = historical_drawdown_frequency(closes, config.drawdown_thresholds_pct)
                history_blocks.append(
                    format_historical_frequency(
                        name, config.drawdown_thresholds_pct, freq, len(closes)
                    )
                )

    fear_greed = fetch_fear_greed()
    fear_greed_line = format_fear_greed(fear_greed)

    # Use earliest data timestamp for "Updated:" display
    data_timestamp = min(data_timestamps) if data_timestamps else datetime.now(UTC)

    report = format_daily_report(
        index_blocks,
        fear_greed_line,
        history_blocks,
        data_timestamp,
        history_years=config.history_years,
    )

    # Add warning if serving stale data
    if has_stale_data:
        report = (
            "⚠️ <i>Some data may be outdated due to API issues. "
            "Showing most recent available data.</i>\n\n" + report
        )

    return report


async def send_daily_report(app: Application[Any, Any, Any, Any, Any, Any], config: Config) -> None:
    """Scheduled job: send daily report to all active subscribers."""
    # Get subscribers from database (falls back to .env for backward compatibility)
    subscribers = database.get_active_subscribers()
    if not subscribers and config.chat_ids:
        logger.info("No active subscribers in DB, using .env chat_ids")
        subscribers = config.chat_ids

    if not subscribers:
        logger.warning("No subscribers configured; skipping daily report")
        return

    logger.info("Generating daily report...")
    report = await asyncio.to_thread(_build_daily_report, config)
    logger.info("Daily report generated successfully")

    sent_count = 0
    for chat_id in subscribers:
        try:
            await app.bot.send_message(chat_id=chat_id, text=report, parse_mode="HTML")
            database.update_last_daily_sent(chat_id)
            logger.info("Daily report sent to chat_id=%s", chat_id)
            sent_count += 1
        except Exception as e:
            logger.exception("Failed to send daily report to %s: %s", chat_id, e)

    logger.info("Daily report sent to %d/%d subscribers", sent_count, len(subscribers))


def _check_drawdown_alerts(config: Config, subscribers: list[str]) -> list[tuple[str, str]]:
    """Check all indices for threshold breaches; return list of (chat_id, message) to send."""
    results: list[tuple[str, str]] = []
    for symbol, name in config.index_symbols.items():
        result = get_index_metrics(symbol, name, years=config.history_years)
        if not result:
            continue
        metrics, _, is_stale = result
        # Skip alerts if data is stale to avoid false alarms
        if is_stale:
            logger.warning("Skipping alert check for %s - data is stale", symbol)
            continue
        closes, _, _ = fetch_index_history(symbol, years=config.history_years)
        total_days = len(closes)
        alert_state.on_drawdown_improved(
            symbol, metrics.current_drawdown_pct, config.drawdown_thresholds_pct
        )
        for threshold in config.drawdown_thresholds_pct:
            if not alert_state.should_alert(symbol, threshold, metrics.current_drawdown_pct):
                continue
            freq = historical_drawdown_frequency(closes, (threshold,))
            day_count = freq.get(threshold, 0)
            msg = format_drawdown_alert(
                name,
                metrics.current_drawdown_pct,
                threshold,
                day_count,
                total_days,
                history_years=config.history_years,
            )
            for chat_id in subscribers:
                results.append((chat_id, msg))
            alert_state.mark_sent(symbol, threshold)
    return results


async def check_and_send_alerts(
    app: Application[Any, Any, Any, Any, Any, Any], config: Config
) -> None:
    """Scheduled job: check drawdown thresholds and send alerts."""
    # Get subscribers from database (falls back to .env for backward compatibility)
    subscribers = database.get_active_subscribers()
    if not subscribers and config.chat_ids:
        subscribers = config.chat_ids

    if not subscribers:
        return

    logger.info("Checking drawdown alerts...")
    to_send = await asyncio.to_thread(_check_drawdown_alerts, config, subscribers)

    if not to_send:
        logger.info("No alerts to send")
        # Save alert state even if no alerts
        try:
            database.save_alert_state(alert_state.sent)
        except Exception as e:
            logger.warning("Failed to save alert state: %s", e)
        return

    logger.info("Sending %d alert(s)...", len(to_send))
    sent_count = 0
    for chat_id, text in to_send:
        try:
            await app.bot.send_message(chat_id=chat_id, text=text, parse_mode="HTML")
            database.update_last_alert_sent(chat_id)
            logger.info("Alert sent to chat_id=%s", chat_id)
            sent_count += 1
        except Exception as e:
            logger.exception("Failed to send alert to %s: %s", chat_id, e)

    logger.info("Sent %d/%d alerts successfully", sent_count, len(to_send))

    # Save alert state to database
    try:
        database.save_alert_state(alert_state.sent)
    except Exception as e:
        logger.warning("Failed to save alert state: %s", e)


async def cmd_start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /start."""
    if not update.message:
        return
    await update.message.reply_text(
        "📈 <b>Index Watch</b> — crash-buy helper\n\n"
        "<b>Commands:</b>\n"
        "• /subscribe — Get daily reports and drawdown alerts\n"
        "• /unsubscribe — Stop receiving notifications\n"
        "• /status — Check your subscription status\n"
        "• /daily — Get today's drawdown report\n"
        "• /alerts — Show configured thresholds\n\n"
        "<i>Use /subscribe to start receiving notifications!</i>",
        parse_mode="HTML",
    )


async def cmd_daily(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /daily — manual daily report with rate limiting."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)
    config = context.bot_data.get("config")

    if not config:
        await update.message.reply_text("⚠️ Bot is starting up, please try again in a moment.")
        return

    # Rate limiting: 5 minutes per user
    remaining = rate_limiter.check_rate_limit(chat_id, "daily", RATE_LIMITS["daily"])
    if remaining is not None:
        minutes = remaining // 60
        seconds = remaining % 60
        await update.message.reply_text(
            f"⏱ Please wait {minutes}m {seconds}s before requesting another report.\n\n"
            "This helps protect our API quota. Use /status to see scheduled report times.",
            parse_mode="HTML",
        )
        logger.info("Rate limited /daily for user %s (%ds remaining)", chat_id, remaining)
        return

    try:
        report = await asyncio.to_thread(_build_daily_report, config)
        await update.message.reply_text(report, parse_mode="HTML")
        logger.info("Sent /daily report to user %s", chat_id)
    except Exception:
        logger.exception("Failed to generate daily report for %s", chat_id)
        await update.message.reply_text(
            "❌ Failed to fetch market data. This usually means Yahoo Finance is "
            "temporarily unavailable. Please try again in a few minutes."
        )


async def cmd_alerts(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /alerts — show configured thresholds."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)

    # Rate limiting: 10 seconds
    remaining = rate_limiter.check_rate_limit(chat_id, "alerts", RATE_LIMITS["alerts"])
    if remaining is not None:
        await update.message.reply_text(f"⏱ Please wait {remaining}s before using /alerts again.")
        return

    config = context.bot_data.get("config")
    if not config:
        await update.message.reply_text("Config not loaded.")
        return

    th = ", ".join(str(t) + "%" for t in config.drawdown_thresholds_pct)
    parts = [f"Drawdown alert thresholds: {th}"]
    parts.append(f"Indices: {', '.join(config.index_symbols.values())}")
    parts.append(f"Alert check interval: every {config.alert_check_minutes} minutes")
    await update.message.reply_text("\n".join(parts))


async def cmd_subscribe(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /subscribe — subscribe to daily reports and alerts."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)
    username = update.message.from_user.username if update.message.from_user else None

    # Rate limiting: 1 minute
    remaining = rate_limiter.check_rate_limit(chat_id, "subscribe", RATE_LIMITS["subscribe"])
    if remaining is not None:
        await update.message.reply_text(
            f"⏱ Please wait {remaining}s before using /subscribe again."
        )
        return

    try:
        is_new = database.add_subscriber(chat_id, username)
        if is_new:
            await update.message.reply_text(
                "✅ <b>You're subscribed!</b>\n\n"
                "You'll receive:\n"
                "• Daily reports at 22:00 UTC (Mon-Fri)\n"
                "• Real-time drawdown alerts (5%, 10%, 15%, 20%)\n\n"
                "Use /unsubscribe to stop notifications anytime.\n"
                "Use /status to check your subscription.",
                parse_mode="HTML",
            )
            logger.info("User %s subscribed", chat_id)
        else:
            await update.message.reply_text(
                "ℹ️ You're already subscribed!\n\nUse /status to check your subscription details.",
                parse_mode="HTML",
            )
    except Exception as e:
        logger.exception("Failed to subscribe user %s: %s", chat_id, e)
        await update.message.reply_text(
            "❌ Failed to subscribe. Please try again later or contact support."
        )


async def cmd_unsubscribe(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /unsubscribe — unsubscribe from notifications."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)

    # Rate limiting: 1 minute
    remaining = rate_limiter.check_rate_limit(chat_id, "unsubscribe", RATE_LIMITS["unsubscribe"])
    if remaining is not None:
        await update.message.reply_text(
            f"⏱ Please wait {remaining}s before using /unsubscribe again."
        )
        return

    try:
        success = database.remove_subscriber(chat_id)
        if success:
            await update.message.reply_text(
                "👋 <b>You've been unsubscribed.</b>\n\n"
                "You'll no longer receive daily reports or alerts.\n\n"
                "Use /subscribe to re-enable notifications anytime.",
                parse_mode="HTML",
            )
            logger.info("User %s unsubscribed", chat_id)
        else:
            await update.message.reply_text(
                "ℹ️ You're not currently subscribed.\n\n"
                "Use /subscribe to start receiving notifications.",
                parse_mode="HTML",
            )
    except Exception as e:
        logger.exception("Failed to unsubscribe user %s: %s", chat_id, e)
        await update.message.reply_text("❌ Failed to unsubscribe. Please try again later.")


async def cmd_status(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /status — show subscription status."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)

    # Rate limiting: 10 seconds
    remaining = rate_limiter.check_rate_limit(chat_id, "status", RATE_LIMITS["status"])
    if remaining is not None:
        await update.message.reply_text(f"⏱ Please wait {remaining}s before using /status again.")
        return

    config = context.bot_data.get("config")
    scheduler = context.bot_data.get("scheduler")

    try:
        is_subscribed = database.is_subscribed(chat_id)

        lines = ["<b>📊 Your Subscription Status</b>\n"]

        if is_subscribed:
            lines.append("✅ <b>Status:</b> Subscribed")

            # Get next report time
            if scheduler:
                daily_job = scheduler.get_job("daily_report")
                if daily_job and daily_job.next_run_time:
                    lines.append(f"📅 <b>Next daily report:</b> {daily_job.next_run_time}")

            # Alert info
            if config:
                thresholds = ", ".join(f"{t}%" for t in config.drawdown_thresholds_pct)
                lines.append(f"🔔 <b>Alert thresholds:</b> {thresholds}")
                lines.append(f"⏱ <b>Check interval:</b> Every {config.alert_check_minutes} min")

            lines.append("\nUse /unsubscribe to stop notifications")
        else:
            lines.append("❌ <b>Status:</b> Not subscribed")
            lines.append("\nUse /subscribe to start receiving notifications")

        await update.message.reply_text("\n".join(lines), parse_mode="HTML")
    except Exception as e:
        logger.exception("Failed to get status for user %s: %s", chat_id, e)
        await update.message.reply_text("❌ Failed to retrieve status. Please try again later.")


async def cmd_mystats(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /mystats — show personal notification history."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)

    # Rate limiting: 10 seconds
    remaining = rate_limiter.check_rate_limit(chat_id, "mystats", RATE_LIMITS["mystats"])
    if remaining is not None:
        await update.message.reply_text(f"⏱ Please wait {remaining}s before using /mystats again.")
        return

    try:
        stats = database.get_subscriber_stats(chat_id)

        if not stats:
            await update.message.reply_text(
                "ℹ️ You're not subscribed.\n\nUse /subscribe to start receiving notifications.",
                parse_mode="HTML",
            )
            return

        lines = ["<b>📈 Your Subscription Stats</b>\n"]

        if stats["subscribed_at"]:
            lines.append(f"📅 <b>Subscribed since:</b> {stats['subscribed_at']}")

        if stats["last_daily_sent"]:
            lines.append(f"📊 <b>Last daily report:</b> {stats['last_daily_sent']}")
        else:
            lines.append("📊 <b>Last daily report:</b> Not yet received")

        if stats["last_alert_sent"]:
            lines.append(f"🔔 <b>Last alert:</b> {stats['last_alert_sent']}")
        else:
            lines.append("🔔 <b>Last alert:</b> None sent")

        status_emoji = "✅" if stats["active"] else "❌"
        status_text = "Active" if stats["active"] else "Inactive"
        lines.append(f"\n{status_emoji} <b>Status:</b> {status_text}")

        await update.message.reply_text("\n".join(lines), parse_mode="HTML")
    except Exception as e:
        logger.exception("Failed to get stats for user %s: %s", chat_id, e)
        await update.message.reply_text("❌ Failed to retrieve stats. Please try again later.")


async def cmd_debug(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /debug — show scheduler and system status (admin only)."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)
    config = context.bot_data.get("config")

    # Admin check
    if config and config.admin_chat_ids:
        if chat_id not in config.admin_chat_ids:
            logger.warning("Unauthorized /debug attempt from user %s", chat_id)
            await update.message.reply_text("⛔️ This command is restricted to administrators.")
            return

    # Rate limiting: 1 minute
    remaining = rate_limiter.check_rate_limit(chat_id, "debug", RATE_LIMITS["debug"])
    if remaining is not None:
        await update.message.reply_text(f"⏱ Please wait {remaining}s before using /debug again.")
        return

    scheduler = context.bot_data.get("scheduler")

    if not scheduler:
        await update.message.reply_text("Scheduler not initialized")
        return

    lines = ["<b>🔧 Debug Info</b>\n"]
    lines.append("<b>Scheduler Status</b>")
    lines.append(f"Running: {'✅ Yes' if scheduler.running else '❌ No'}")

    jobs = scheduler.get_jobs()
    lines.append(f"\n<b>Jobs: {len(jobs)}</b>")

    for job in jobs:
        lines.append(f"\n{job.id}:")
        lines.append(f"  Next run: {job.next_run_time}")
        lines.append(f"  Trigger: {job.trigger}")

    if config:
        lines.append("\n<b>Configuration</b>")
        db_stats = database.get_db_stats()
        active = db_stats["active_subscribers"]
        total = db_stats["total_subscribers"]
        lines.append(f"Subscribers: {active} active / {total} total")
        lines.append(f".env chat_ids: {len(config.chat_ids)}")
        lines.append(f"Indices: {len(config.index_symbols)}")
        lines.append(f"Alert thresholds: {len(config.drawdown_thresholds_pct)}")

    lines.append("\n<b>Alert State</b>")
    lines.append(f"Active alerts: {len(alert_state.sent)}")

    # Cache stats
    cache = get_cache()
    cache_stats = cache.get_stats()
    lines.append("\n<b>Cache Stats</b>")
    lines.append(f"Entries: {cache_stats['entries']}")
    lines.append(f"Hits: {cache_stats['hits']} | Misses: {cache_stats['misses']}")
    lines.append(f"Hit rate: {cache_stats['hit_rate_pct']}%")

    await update.message.reply_text("\n".join(lines), parse_mode="HTML")


async def cmd_clearcache(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /clearcache — clear the in-memory data cache (admin only)."""
    if not update.message:
        return

    chat_id = str(update.message.chat_id)
    config = context.bot_data.get("config")

    # Admin check
    if config and config.admin_chat_ids:
        if chat_id not in config.admin_chat_ids:
            logger.warning("Unauthorized /clearcache attempt from user %s", chat_id)
            await update.message.reply_text("⛔️ This command is restricted to administrators.")
            return

    # Rate limiting: 30 seconds
    remaining = rate_limiter.check_rate_limit(chat_id, "clearcache", RATE_LIMITS["clearcache"])
    if remaining is not None:
        await update.message.reply_text(
            f"⏱ Please wait {remaining}s before using /clearcache again."
        )
        return

    cache = get_cache()
    stats_before = cache.get_stats()
    entries_cleared = stats_before["entries"]
    cache.clear()

    await update.message.reply_text(
        f"🗑 <b>Cache cleared.</b>\n\n"
        f"Removed {entries_cleared} cached {'entry' if entries_cleared == 1 else 'entries'}.\n"
        f"Fresh data will be fetched on the next request.",
        parse_mode="HTML",
    )
    logger.info("Cache cleared by admin %s (%d entries removed)", chat_id, entries_cleared)


def setup_scheduler(
    app: Application[Any, Any, Any, Any, Any, Any], config: Config
) -> AsyncIOScheduler:
    """Add scheduled jobs for daily report and drawdown checks."""
    scheduler = AsyncIOScheduler()

    # Add event listeners for job execution
    def job_executed(event):
        logger.info("Job '%s' executed successfully", event.job_id)

    def job_error(event):
        logger.error("Job '%s' raised exception: %s", event.job_id, event.exception)

    scheduler.add_listener(job_executed, EVENT_JOB_EXECUTED)
    scheduler.add_listener(job_error, EVENT_JOB_ERROR)

    cron_kw = _cron_from_cronstr(config.daily_report_cron) or {"hour": 22, "minute": 0}
    scheduler.add_job(
        send_daily_report,
        "cron",
        args=[app, config],
        id="daily_report",
        **cron_kw,
    )
    logger.info("Scheduled daily report: cron=%s", config.daily_report_cron)

    scheduler.add_job(
        check_and_send_alerts,
        "interval",
        args=[app, config],
        id="alert_check",
        minutes=config.alert_check_minutes,
    )
    logger.info("Scheduled alert checks: every %d minutes", config.alert_check_minutes)

    app.bot_data["scheduler"] = scheduler
    return scheduler


async def _on_application_ready(application: Application[Any, Any, Any, Any, Any, Any]) -> None:
    """Run when the app is ready (event loop is up): start the scheduler."""
    scheduler = application.bot_data.get("scheduler")
    if scheduler is not None:
        scheduler.start()
        jobs = scheduler.get_jobs()
        logger.info("Scheduler started with %d job(s)", len(jobs))


def _cron_from_cronstr(cron: str) -> dict[str, Any]:
    """Parse cron 'minute hour day month weekday' into apscheduler kwargs (non-wildcard only)."""
    parts = cron.split()
    if len(parts) != 5:
        return {}
    minute, hour, day, month, weekday = parts
    out: dict[str, Any] = {}
    if minute != "*":
        out["minute"] = minute
    if hour != "*":
        out["hour"] = hour
    if day != "*":
        out["day"] = day
    if month != "*":
        out["month"] = month
    if weekday != "*":
        out["day_of_week"] = weekday
    return out


def build_application(config: Config) -> Application[Any, Any, Any, Any, Any, Any]:
    """Create and configure the Telegram application."""
    app = (
        Application.builder()
        .token(config.telegram_bot_token)
        .post_init(_on_application_ready)
        .build()
    )
    app.bot_data["config"] = config
    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("subscribe", cmd_subscribe))
    app.add_handler(CommandHandler("unsubscribe", cmd_unsubscribe))
    app.add_handler(CommandHandler("status", cmd_status))
    app.add_handler(CommandHandler("mystats", cmd_mystats))
    app.add_handler(CommandHandler("daily", cmd_daily))
    app.add_handler(CommandHandler("alerts", cmd_alerts))
    app.add_handler(CommandHandler("debug", cmd_debug))
    app.add_handler(CommandHandler("clearcache", cmd_clearcache))
    return app
