"""Format drawdown and alert messages for Telegram."""

from datetime import UTC, datetime
from zoneinfo import ZoneInfo

from index_watch.drawdown import DrawdownMetrics
from index_watch.fear_greed import FearGreedResult

# Default display timezone (GMT+8)
DEFAULT_DISPLAY_TZ = ZoneInfo("Asia/Singapore")


def get_drawdown_emoji(drawdown_pct: float) -> str:
    """Get status emoji based on drawdown severity."""
    if drawdown_pct >= -5:
        return "🟢"  # Healthy: 0% to -5%
    elif drawdown_pct >= -10:
        return "🟡"  # Caution: -5% to -10%
    elif drawdown_pct >= -15:
        return "🟠"  # Warning: -10% to -15%
    elif drawdown_pct >= -20:
        return "🔴"  # Severe: -15% to -20%
    else:
        return "🚨"  # Extreme: > -20%


def get_fear_greed_emoji(value: float) -> str:
    """Get emoji based on Fear & Greed Index value."""
    if value < 25:
        return "😱"  # Extreme Fear
    elif value < 45:
        return "😨"  # Fear
    elif value < 55:
        return "😐"  # Neutral
    elif value < 75:
        return "😃"  # Greed
    else:
        return "🤑"  # Extreme Greed


def format_timestamp_gmt8(dt: datetime) -> str:
    """Format datetime in GMT+8 timezone."""
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)
    local_dt = dt.astimezone(DEFAULT_DISPLAY_TZ)
    return local_dt.strftime("%Y-%m-%d %H:%M GMT+8")


def format_drawdown_block(name: str, m: DrawdownMetrics) -> str:
    """Format one index's drawdown metrics with emoji indicators."""
    emoji = get_drawdown_emoji(m.current_drawdown_pct)

    return f"""<b>📊 {name}</b> {emoji}
━━━━━━━━━━━━━━━━━
<b>Current Status</b>
• Current Drawdown: <b>{m.current_drawdown_pct:.2f}%</b> {emoji}
• Last Closing Price: {m.current_price:,.2f}
• All Time High: {m.ath:,.2f}

<b>Historical Context</b>
• Lowest Since ATH: {m.lowest_since_ath:,.2f}
• Drawdown at Lowest: {m.drawdown_at_lowest_pct:.2f}%

<b>Recovery Metrics</b>
• Gain From Lowest: <b>+{m.gain_from_lowest_pct:.2f}%</b>
• Gain Needed (Current → ATH): <b>+{m.gain_to_ath_from_current_pct:.2f}%</b>
• Gain Needed (Lowest → ATH): +{m.gain_to_ath_from_lowest_pct:.2f}%"""


def format_fear_greed(fg: FearGreedResult | None) -> str:
    """Format Fear & Greed line for daily report with emoji."""
    if fg is None:
        return "😐 CNN Fear & Greed Index: unavailable"
    emoji = get_fear_greed_emoji(fg.value)
    return (
        f"{emoji} <b>Fear & Greed Index:</b> {fg.value:.1f} — "
        f"{fg.description} (updated {fg.last_update})"
    )


def format_historical_frequency(
    name: str,
    thresholds_pct: tuple[int, ...],
    day_counts: dict[int, int],
    total_days: int,
) -> str:
    """Format historical drawdown frequency with colored indicators."""
    lines = [f"<b>{name}</b> — historical trading days at or below drawdown:"]
    for t in thresholds_pct:
        count = day_counts.get(t, 0)
        pct = (count / total_days * 100) if total_days else 0
        # Color indicators based on threshold severity
        if t <= 5:
            indicator = "🟢"
        elif t <= 10:
            indicator = "🟡"
        elif t <= 15:
            indicator = "🟠"
        else:
            indicator = "🔴"
        lines.append(f"  {indicator} {t}%: {count} days ({pct:.1f}% of history)")
    return "\n".join(lines)


def format_daily_report(
    index_blocks: list[tuple[str, str]],  # (name, formatted_block)
    fear_greed_line: str,
    history_blocks: list[str],  # optional historical frequency per index
    data_timestamp: datetime | None = None,
) -> str:
    """Assemble full daily report message with timestamp header."""
    parts = ["<b>📈 Daily Index Watch</b>"]

    # Add data timestamp if provided
    if data_timestamp:
        timestamp_str = format_timestamp_gmt8(data_timestamp)
        parts.append(f"<i>Updated: {timestamp_str}</i>")

    parts.append("")  # Blank line after header

    for name, block in index_blocks:
        parts.append(block)
        parts.append("")

    parts.append(fear_greed_line)

    if history_blocks:
        parts.append("")
        parts.append("<b>📊 Historical Drawdown Frequency</b>")
        parts.append("<i>(Last 20 years)</i>")
        for block in history_blocks:
            parts.append(block)
            parts.append("")  # Spacing between indices

    return "\n".join(parts).strip()


def format_drawdown_alert(
    name: str,
    current_drawdown_pct: float,
    threshold_pct: int,
    day_count: int,
    total_days: int,
) -> str:
    """Format alert when drawdown crosses a threshold with emoji indicators."""
    pct_of_history = (day_count / total_days * 100) if total_days else 0
    emoji = get_drawdown_emoji(current_drawdown_pct)

    return (
        f"🚨 <b>Drawdown Alert: {name}</b> {emoji}\n\n"
        f"📉 <b>Current:</b> {current_drawdown_pct:.2f}% (crossed -{threshold_pct}% threshold)\n"
        f"Status: {emoji}\n\n"
        f"<b>📊 Historical Context</b>\n"
        f"In the last 20 years, we've seen -{threshold_pct}% or worse on:\n"
        f"• {day_count} trading days ({pct_of_history:.1f}% of ~{total_days} days)\n\n"
        f"💡 <i>This is {'a relatively rare' if pct_of_history < 5 else 'not uncommon'} event.</i>"
    )
