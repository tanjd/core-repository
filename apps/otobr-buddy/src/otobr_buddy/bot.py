"""Telegram bot handlers."""

import logging
import re
import secrets
from datetime import timedelta
from typing import Any
from zoneinfo import ZoneInfo

from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import (
    Application,
    CallbackQueryHandler,
    CommandHandler,
    ContextTypes,
)
from telegram_bot_shared.env import is_admin

from otobr_buddy import database, reminders, stats
from otobr_buddy.config import Config
from otobr_buddy.timeutil import parse_timestamp, utcnow

logger = logging.getLogger(__name__)

INVITE_TTL_HOURS = 24
PAIR_TTL_HOURS = 24
# Excludes 0/O/1/I/L to avoid ambiguity when read aloud or typed by hand.
_INVITE_CODE_ALPHABET = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
_INVITE_CODE_LENGTH = 6

_DAY_NAMES = {"mon": 0, "tue": 1, "wed": 2, "thu": 3, "fri": 4, "sat": 5, "sun": 6}
_DAY_LABELS = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]
_TIME_RE = re.compile(r"^([01]\d|2[0-3]):([0-5]\d)$")

HELP_TEXT = (
    "\U0001f4d6 <b>otobr-buddy</b> — one-to-one Bible reading companion\n\n"
    "<b>Getting paired:</b>\n"
    "/pair — add me to a group chat with your reading partner and both run this "
    "there to pair up (no code needed) — <i>group chats only</i>\n"
    "/invite, /join &lt;code&gt; — pair by DM instead, if you'd rather not share a group "
    "— <i>DM only</i>\n\n"
    "<b>Once paired:</b>\n"
    "/partners — list your active reading partnerships — <i>DM only</i>\n"
    "/log &lt;text&gt; — log a reading session, e.g. /log Romans 8:1-17\n"
    "/setfrequency — set a reminder schedule for a partnership\n"
    "/link — link the current group chat to a DM-formed partnership — <i>group chats only</i>\n"
    "/end — end a partnership\n"
    "/history — view past partnerships — <i>DM only</i>\n"
    "/stats — session counts, streaks, and coverage\n\n"
    "If you have more than one active partnership, prefix a command with which one, "
    "e.g. <code>/log #2 Romans 8:1-17</code> (see /partners for the numbers)."
)


# ---------------------------------------------------------------------------
# Small helpers
# ---------------------------------------------------------------------------


def _upsert_from_update(update: Update) -> str:
    user = update.effective_user
    assert user is not None
    telegram_id = str(user.id)
    database.upsert_user(telegram_id, user.username, user.first_name)
    return telegram_id


def _partner_label(user: dict[str, Any] | None, fallback_id: str) -> str:
    if not user:
        return f"user {fallback_id}"
    if user.get("username"):
        return f"@{user['username']}"
    if user.get("first_name"):
        return str(user["first_name"])
    return f"user {fallback_id}"


def _frequency_label(partnership: dict[str, Any]) -> str:
    mode = partnership.get("frequency_mode")
    if mode == database.FREQUENCY_INTERVAL:
        return f"every {partnership['frequency_interval_days']} day(s)"
    if mode == database.FREQUENCY_WEEKLY:
        day_name = _DAY_LABELS[partnership["frequency_day_of_week"]]
        return f"every {day_name} at {partnership['frequency_time']}"
    return "no reminder set yet (use /setfrequency)"


def _partnership_summary(partnership: dict[str, Any], viewer_id: str) -> str:
    other_id = database.other_user_id(partnership, viewer_id)
    other = database.get_user(other_id)
    label = _partner_label(other, other_id)
    chat = "in a linked group chat" if partnership.get("group_chat_id") else "via DM"
    return f"{label} — {_frequency_label(partnership)} — {chat}"


def _ambiguous_partnership_error(partnerships: list[dict[str, Any]], viewer_id: str) -> str:
    lines = ["You have multiple active partnerships — say which one first:"]
    for i, p in enumerate(partnerships, start=1):
        lines.append(f"#{i}. {_partnership_summary(p, viewer_id)}")
    lines.append("\nPrefix your command with the number, e.g. /log #1 Romans 8:1-17")
    return "\n".join(lines)


def _select_partnership(
    args: list[str], partnerships: list[dict[str, Any]], viewer_id: str
) -> tuple[dict[str, Any] | None, list[str], str | None]:
    """Resolve which partnership a command applies to.

    With exactly one active partnership it's used implicitly. With more than one,
    the first arg must be a "#N" token (not a bare number — many Bible books start
    with a digit, e.g. "1 Corinthians", so a bare number would be ambiguous with
    the reading text itself).
    """
    if not partnerships:
        return (
            None,
            args,
            "You don't have any active reading partnerships yet. Use /pair in a shared "
            "group chat, or /invite for a DM-based pairing.",
        )

    if len(partnerships) == 1:
        return partnerships[0], args, None

    if not args or not (args[0].startswith("#") and args[0][1:].isdigit()):
        return None, args, _ambiguous_partnership_error(partnerships, viewer_id)

    index = int(args[0][1:])
    if not 1 <= index <= len(partnerships):
        return None, args, _ambiguous_partnership_error(partnerships, viewer_id)

    return partnerships[index - 1], args[1:], None


def _generate_invite_code() -> str:
    return "".join(secrets.choice(_INVITE_CODE_ALPHABET) for _ in range(_INVITE_CODE_LENGTH))


async def _require_private_chat(update: Update, hint: str) -> bool:
    """True if the command may proceed; sends `hint` and returns False if run in a group."""
    assert update.message is not None
    if update.effective_chat and update.effective_chat.type in ("group", "supergroup"):
        await update.message.reply_text(hint)
        return False
    return True


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------


async def cmd_start(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    _upsert_from_update(update)
    await update.message.reply_text(HELP_TEXT, parse_mode="HTML")


async def cmd_pair(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message or not update.effective_chat:
        return
    if update.effective_chat.type not in ("group", "supergroup"):
        await update.message.reply_text(
            "Add me to a group chat with your reading partner and both run /pair there — "
            "no invite code needed. Prefer to pair by DM instead? Use /invite."
        )
        return

    user_id = _upsert_from_update(update)
    group_chat_id = str(update.effective_chat.id)

    existing = database.get_active_partnership_for_group(group_chat_id)
    if existing:
        await update.message.reply_text(
            "This group is already linked to a reading partnership. Use /partners to see it."
        )
        return

    pending = database.get_pending_pair(group_chat_id)
    if pending and parse_timestamp(pending["expires_at"]) >= utcnow():
        if pending["claimant_id"] == user_id:
            await update.message.reply_text(
                "Still waiting for your reading partner to also send /pair in this chat."
            )
            return

        database.clear_pending_pair(group_chat_id)
        partnership_id = database.create_partnership(
            pending["claimant_id"], user_id, group_chat_id=group_chat_id
        )
        partner = database.get_user(pending["claimant_id"])
        logger.info("Formed partnership %d via /pair in group %s", partnership_id, group_chat_id)
        await update.message.reply_text(
            f"✅ You're now reading with {_partner_label(partner, pending['claimant_id'])}!\n\n"
            "Set a reminder schedule with /setfrequency, and log sessions with /log — "
            "right here in this chat.",
            parse_mode="HTML",
        )
        return

    database.create_pending_pair(group_chat_id, user_id, utcnow() + timedelta(hours=PAIR_TTL_HOURS))
    await update.message.reply_text(
        "\U0001f4d6 Got it — waiting for your reading partner to also send /pair in this chat "
        f"to complete the pairing (expires in {PAIR_TTL_HOURS} hours)."
    )


async def cmd_invite(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    if not await _require_private_chat(
        update,
        "Message me privately for an invite code — pairing with a shared group instead? "
        "Use /pair there.",
    ):
        return
    user_id = _upsert_from_update(update)
    code = _generate_invite_code()
    database.create_invite(code, user_id, utcnow() + timedelta(hours=INVITE_TTL_HOURS))
    await update.message.reply_text(
        f"✉️ Invite code: <code>{code}</code>\n\n"
        f"Share it with your reading partner. They should message me privately with:\n"
        f"<code>/join {code}</code>\n\n"
        f"Expires in {INVITE_TTL_HOURS} hours.\n\n"
        f"Tip: sharing a group chat with them instead? Add me to it and both run /pair there.",
        parse_mode="HTML",
    )


async def cmd_join(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message or not update.effective_user:
        return
    if not await _require_private_chat(
        update,
        "Message me privately to redeem an invite code — pairing with a shared group instead? "
        "Use /pair there.",
    ):
        return
    if not context.args:
        await update.message.reply_text("Usage: /join <code>")
        return

    user_id = _upsert_from_update(update)
    code = context.args[0].strip().upper()
    invite = database.get_invite(code)

    if not invite:
        await update.message.reply_text("❌ Invite code not found.")
        return
    if invite["used_by"]:
        await update.message.reply_text("❌ That invite has already been used.")
        return
    if parse_timestamp(invite["expires_at"]) < utcnow():
        await update.message.reply_text(
            "❌ That invite has expired. Ask for a new one with /invite."
        )
        return
    if invite["created_by"] == user_id:
        await update.message.reply_text("❌ You can't redeem your own invite.")
        return

    database.mark_invite_used(code, user_id)
    database.create_partnership(invite["created_by"], user_id)

    inviter = database.get_user(invite["created_by"])
    await update.message.reply_text(
        f"✅ You're now reading with {_partner_label(inviter, invite['created_by'])}!\n\n"
        "Set a reminder schedule with /setfrequency, and log sessions with /log.",
        parse_mode="HTML",
    )
    try:
        joiner_name = update.effective_user.first_name or "Your partner"
        await context.bot.send_message(
            chat_id=invite["created_by"],
            text=f"✅ {joiner_name} joined using your invite! You're now reading together.",
        )
    except Exception:
        logger.exception("Failed to notify inviter %s", invite["created_by"])


async def cmd_partners(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    if not await _require_private_chat(
        update,
        "Message me privately to see your partnerships — the list can include partnerships "
        "beyond this chat.",
    ):
        return
    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    if not partnerships:
        await update.message.reply_text(
            "You don't have any active reading partnerships yet. Use /pair in a shared "
            "group chat, or /invite for a DM-based pairing."
        )
        return

    lines = ["<b>Your active partnerships:</b>"]
    for i, p in enumerate(partnerships, start=1):
        lines.append(f"#{i}. {_partnership_summary(p, user_id)}")
    if len(partnerships) > 1:
        lines.append("\nPrefix other commands with #1, #2, etc., e.g. /log #2 Romans 8:1-17")
    await update.message.reply_text("\n".join(lines), parse_mode="HTML")


async def cmd_log(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    partnership, rest, error = _select_partnership(context.args or [], partnerships, user_id)
    if error:
        await update.message.reply_text(error)
        return
    assert partnership is not None

    text_covered = " ".join(rest).strip()
    if not text_covered:
        await update.message.reply_text("Usage: /log <what you covered>, e.g. /log Romans 8:1-17")
        return

    database.add_session(partnership["id"], text_covered, user_id)

    if partnership.get("frequency_mode") == database.FREQUENCY_INTERVAL:
        updated = database.get_partnership(partnership["id"])
        assert updated is not None
        assert context.job_queue is not None
        reminders.schedule_partnership_reminder(
            context.job_queue, updated, context.bot_data["timezone"]
        )

    await update.message.reply_text(f"✅ Logged: {text_covered}")


async def cmd_setfrequency(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    partnership, rest, error = _select_partnership(context.args or [], partnerships, user_id)
    if error:
        await update.message.reply_text(error)
        return
    assert partnership is not None

    usage = (
        "Usage:\n"
        "/setfrequency interval <days> — remind N days after the last session\n"
        "/setfrequency weekly <mon..sun> <HH:MM> — remind on a fixed day/time\n"
        "(prefix with #1, #2, etc. first if you have more than one partnership)"
    )
    if len(rest) < 2:
        await update.message.reply_text(usage)
        return

    kind = rest[0].lower()
    tz: ZoneInfo = context.bot_data["timezone"]
    assert context.job_queue is not None

    if kind == "interval":
        if not rest[1].isdigit() or not 1 <= int(rest[1]) <= 365:
            await update.message.reply_text("Days must be a whole number between 1 and 365.")
            return
        days = int(rest[1])
        database.set_frequency_interval(partnership["id"], days)
        updated = database.get_partnership(partnership["id"])
        assert updated is not None
        reminders.schedule_partnership_reminder(context.job_queue, updated, tz)
        await update.message.reply_text(
            f"✅ Reminder set: every {days} day(s) after your last session."
        )

    elif kind == "weekly":
        day_key = rest[1].lower() if len(rest) > 1 else ""
        time_str = rest[2] if len(rest) > 2 else ""
        if day_key not in _DAY_NAMES:
            await update.message.reply_text("Day must be one of: mon tue wed thu fri sat sun")
            return
        if not _TIME_RE.match(time_str):
            await update.message.reply_text("Time must be 24h HH:MM, e.g. 19:00")
            return
        day = _DAY_NAMES[day_key]
        database.set_frequency_weekly(partnership["id"], day, time_str)
        updated = database.get_partnership(partnership["id"])
        assert updated is not None
        reminders.schedule_partnership_reminder(context.job_queue, updated, tz)
        await update.message.reply_text(
            f"✅ Reminder set: every {_DAY_LABELS[day]} at {time_str} ({tz})."
        )
    else:
        await update.message.reply_text(usage)


async def cmd_link(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message or not update.effective_chat:
        return
    if update.effective_chat.type not in ("group", "supergroup"):
        await update.message.reply_text(
            "Run /link inside the group chat you share with your reading partner."
        )
        return

    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    partnership, _rest, error = _select_partnership(context.args or [], partnerships, user_id)
    if error:
        await update.message.reply_text(error)
        return
    assert partnership is not None

    group_chat_id = str(update.effective_chat.id)
    existing = database.get_active_partnership_for_group(group_chat_id)
    if existing and existing["id"] != partnership["id"]:
        await update.message.reply_text(
            "This group is already linked to a different reading partnership."
        )
        return

    database.set_group_chat(partnership["id"], group_chat_id)
    await update.message.reply_text(
        "✅ This group is now linked. Reminders and /log for this partnership will use this chat."
    )


async def cmd_end(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    partnership, _rest, error = _select_partnership(context.args or [], partnerships, user_id)
    if error:
        await update.message.reply_text(error)
        return
    assert partnership is not None

    keyboard = InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton(
                    "Yes, end it", callback_data=f"end_confirm:{partnership['id']}"
                ),
                InlineKeyboardButton("Cancel", callback_data="end_cancel"),
            ]
        ]
    )
    await update.message.reply_text(
        f"End this partnership?\n{_partnership_summary(partnership, user_id)}",
        reply_markup=keyboard,
    )


async def on_end_callback(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    query = update.callback_query
    if not query or not query.data or not query.from_user:
        return
    await query.answer()

    if query.data == "end_cancel":
        await query.edit_message_text("Cancelled.")
        return

    partnership_id = int(query.data.split(":", 1)[1])
    partnership = database.get_partnership(partnership_id)
    user_id = str(query.from_user.id)
    if (
        not partnership
        or partnership["status"] != database.STATUS_ACTIVE
        or user_id not in (partnership["user_a_id"], partnership["user_b_id"])
    ):
        await query.edit_message_text("That partnership is no longer active.")
        return

    database.end_partnership(partnership_id)
    assert context.job_queue is not None
    reminders.cancel_reminder(context.job_queue, partnership_id)
    await query.edit_message_text("✅ Partnership ended. It'll show up in /history.")


async def cmd_history(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    if not await _require_private_chat(
        update,
        "Message me privately to see your partnership history — the list can include "
        "partnerships beyond this chat.",
    ):
        return
    user_id = _upsert_from_update(update)
    ended = database.get_ended_partnerships_for_user(user_id)
    if not ended:
        await update.message.reply_text("No past partnerships yet.")
        return

    lines = ["<b>Past partnerships:</b>"]
    for p in ended:
        other_id = database.other_user_id(p, user_id)
        other = database.get_user(other_id)
        label = _partner_label(other, other_id)
        count = database.count_sessions_for_partnership(p["id"])
        lines.append(f"• {label}: {p['started_at']} → {p['ended_at']} ({count} session(s))")
    await update.message.reply_text("\n".join(lines), parse_mode="HTML")


async def cmd_stats(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if not update.message:
        return
    user_id = _upsert_from_update(update)
    partnerships = database.get_active_partnerships_for_user(user_id)
    partnership, _rest, error = _select_partnership(context.args or [], partnerships, user_id)
    if error:
        await update.message.reply_text(error)
        return
    assert partnership is not None

    other_id = database.other_user_id(partnership, user_id)
    other = database.get_user(other_id)
    sessions = database.get_sessions_for_partnership(partnership["id"])

    lines = [
        f"<b>Stats with {_partner_label(other, other_id)}</b>",
        f"Total sessions: {len(sessions)}",
    ]

    cycle_days = stats.cycle_length_days(partnership)
    if cycle_days:
        timestamps = [parse_timestamp(s["logged_at"]) for s in sessions]
        streaks = stats.compute_streaks(timestamps, cycle_days)
        lines.append(f"Current streak: {streaks.current} cycle(s)")
        lines.append(f"Longest streak: {streaks.longest} cycle(s)")
    else:
        lines.append("Streak: set a frequency with /setfrequency to enable streak tracking.")

    coverage = stats.coverage_summary(sessions)
    if coverage:
        lines.append("\n<b>Recent coverage:</b>")
        recent = coverage[-10:]
        for ts, text in recent:
            lines.append(f"• {ts.date()} — {text}")
        if len(coverage) > len(recent):
            lines.append(f"...and {len(coverage) - len(recent)} earlier session(s).")

    await update.message.reply_text("\n".join(lines), parse_mode="HTML")


async def cmd_debug(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Handle /debug — show database-wide stats (admin only)."""
    if not update.message or not update.effective_chat:
        return

    chat_id = str(update.effective_chat.id)
    config: Config = context.bot_data["config"]

    if not is_admin(chat_id, config.admin_chat_ids):
        logger.warning("Unauthorized /debug attempt from user %s", chat_id)
        await update.message.reply_text("⛔️ This command is restricted to administrators.")
        return

    db_stats = database.get_db_stats()
    lines = [
        "<b>🔧 Debug Info</b>\n",
        f"Total users: {db_stats['total_users']}",
        f"Active partnerships: {db_stats['active_partnerships']}",
        f"Ended partnerships: {db_stats['ended_partnerships']}",
        f"Total sessions logged: {db_stats['total_sessions']}",
    ]
    await update.message.reply_text("\n".join(lines), parse_mode="HTML")


# ---------------------------------------------------------------------------
# Application wiring
# ---------------------------------------------------------------------------


def build_application(config: Config) -> Application[Any, Any, Any, Any, Any, Any]:
    """Create and configure the Telegram application."""
    app = Application.builder().token(config.telegram_bot_token).build()
    app.bot_data["config"] = config
    app.bot_data["timezone"] = ZoneInfo(config.timezone)

    app.add_handler(CommandHandler("start", cmd_start))
    app.add_handler(CommandHandler("help", cmd_start))
    app.add_handler(CommandHandler("pair", cmd_pair))
    app.add_handler(CommandHandler("invite", cmd_invite))
    app.add_handler(CommandHandler("join", cmd_join))
    app.add_handler(CommandHandler("partners", cmd_partners))
    app.add_handler(CommandHandler("log", cmd_log))
    app.add_handler(CommandHandler("setfrequency", cmd_setfrequency))
    app.add_handler(CommandHandler("link", cmd_link))
    app.add_handler(CommandHandler("end", cmd_end))
    app.add_handler(CommandHandler("history", cmd_history))
    app.add_handler(CommandHandler("stats", cmd_stats))
    app.add_handler(CommandHandler("debug", cmd_debug))
    app.add_handler(CallbackQueryHandler(on_end_callback, pattern=r"^end_(confirm|cancel)"))

    return app
