"""Shared UTC time helpers.

sqlite's CURRENT_TIMESTAMP default produces naive UTC strings, so the whole app
works in naive UTC datetimes for storage and comparison, and only attaches a real
tzinfo when python-telegram-bot's JobQueue needs one for wall-clock scheduling
(see reminders.py's weekly mode).
"""

from datetime import UTC, datetime


def utcnow() -> datetime:
    """Naive UTC 'now', matching how sqlite stores CURRENT_TIMESTAMP values."""
    return datetime.now(UTC).replace(tzinfo=None)


def parse_timestamp(value: str) -> datetime:
    """Parse a sqlite TIMESTAMP column value (e.g. '2026-07-22 10:30:00'); naive UTC."""
    return datetime.fromisoformat(value)


def format_timestamp(value: datetime) -> str:
    """Render a naive UTC datetime the same way sqlite's CURRENT_TIMESTAMP does."""
    return value.isoformat(sep=" ", timespec="seconds")
