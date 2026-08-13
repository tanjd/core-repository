"""Database operations for users, partnerships, invites, pending pairs, and sessions."""

import logging
import sqlite3
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import datetime
from pathlib import Path
from typing import Any

from otobr_buddy.timeutil import format_timestamp, utcnow

logger = logging.getLogger(__name__)

# Default location; overridden at startup via set_db_path() with the configured path.
DB_PATH = Path("data") / "otobr_buddy.db"

STATUS_ACTIVE = "active"
STATUS_ENDED = "ended"

FREQUENCY_INTERVAL = "interval"
FREQUENCY_WEEKLY = "weekly"


def set_db_path(path: Path) -> None:
    """Override the database file location (call once at startup, or in tests)."""
    global DB_PATH
    DB_PATH = path


@contextmanager
def get_db() -> Iterator[sqlite3.Connection]:
    """Context manager for database connection with auto-commit/rollback."""
    conn = sqlite3.connect(str(DB_PATH))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    try:
        yield conn
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


def init_db() -> None:
    """Initialize database with schema if not exists."""
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)

    with get_db() as conn:
        conn.execute("""
            CREATE TABLE IF NOT EXISTS users (
                telegram_id TEXT PRIMARY KEY,
                username TEXT,
                first_name TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS partnerships (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                user_a_id TEXT NOT NULL REFERENCES users(telegram_id),
                user_b_id TEXT NOT NULL REFERENCES users(telegram_id),
                status TEXT NOT NULL DEFAULT 'active',
                frequency_mode TEXT,
                frequency_interval_days INTEGER,
                frequency_day_of_week INTEGER,
                frequency_time TEXT,
                group_chat_id TEXT,
                started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                ended_at TIMESTAMP
            )
        """)
        conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_partnerships_user_a
            ON partnerships(user_a_id, status)
        """)
        conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_partnerships_user_b
            ON partnerships(user_b_id, status)
        """)
        conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_partnerships_group_chat
            ON partnerships(group_chat_id, status)
        """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS invites (
                code TEXT PRIMARY KEY,
                created_by TEXT NOT NULL REFERENCES users(telegram_id),
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                expires_at TIMESTAMP NOT NULL,
                used_by TEXT,
                used_at TIMESTAMP
            )
        """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS pending_pairs (
                group_chat_id TEXT PRIMARY KEY,
                claimant_id TEXT NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                expires_at TIMESTAMP NOT NULL
            )
        """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS sessions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                partnership_id INTEGER NOT NULL REFERENCES partnerships(id),
                text_covered TEXT NOT NULL,
                logged_by TEXT NOT NULL REFERENCES users(telegram_id),
                logged_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        """)
        conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_sessions_partnership
            ON sessions(partnership_id, logged_at)
        """)

    logger.info("Database initialized at %s", DB_PATH)


# ---------------------------------------------------------------------------
# Users
# ---------------------------------------------------------------------------


def upsert_user(telegram_id: str, username: str | None, first_name: str | None) -> None:
    """Insert a user, or refresh their username/first_name if already known."""
    with get_db() as conn:
        conn.execute(
            """
            INSERT INTO users (telegram_id, username, first_name)
            VALUES (?, ?, ?)
            ON CONFLICT(telegram_id) DO UPDATE SET
                username = excluded.username,
                first_name = excluded.first_name
            """,
            (telegram_id, username, first_name),
        )


def get_user(telegram_id: str) -> dict[str, Any] | None:
    """Fetch a user by Telegram ID."""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM users WHERE telegram_id = ?", (telegram_id,)).fetchone()
        return dict(row) if row else None


# ---------------------------------------------------------------------------
# Invites
# ---------------------------------------------------------------------------


def create_invite(code: str, created_by: str, expires_at: datetime) -> None:
    """Create a new invite code."""
    with get_db() as conn:
        conn.execute(
            "INSERT INTO invites (code, created_by, expires_at) VALUES (?, ?, ?)",
            (code, created_by, format_timestamp(expires_at)),
        )


def get_invite(code: str) -> dict[str, Any] | None:
    """Fetch an invite by code."""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM invites WHERE code = ?", (code,)).fetchone()
        return dict(row) if row else None


def mark_invite_used(code: str, used_by: str) -> None:
    """Mark an invite as redeemed."""
    with get_db() as conn:
        conn.execute(
            "UPDATE invites SET used_by = ?, used_at = ? WHERE code = ?",
            (used_by, format_timestamp(utcnow()), code),
        )


# ---------------------------------------------------------------------------
# Pending pairs (group-based auto-pairing via /pair)
# ---------------------------------------------------------------------------


def create_pending_pair(group_chat_id: str, claimant_id: str, expires_at: datetime) -> None:
    """Record (or overwrite) the first claimant of a group-chat pairing."""
    with get_db() as conn:
        conn.execute(
            """
            INSERT INTO pending_pairs (group_chat_id, claimant_id, expires_at)
            VALUES (?, ?, ?)
            ON CONFLICT(group_chat_id) DO UPDATE SET
                claimant_id = excluded.claimant_id,
                created_at = CURRENT_TIMESTAMP,
                expires_at = excluded.expires_at
            """,
            (group_chat_id, claimant_id, format_timestamp(expires_at)),
        )


def get_pending_pair(group_chat_id: str) -> dict[str, Any] | None:
    """Fetch the pending pairing claim for a group chat, if any."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM pending_pairs WHERE group_chat_id = ?", (group_chat_id,)
        ).fetchone()
        return dict(row) if row else None


def clear_pending_pair(group_chat_id: str) -> None:
    """Remove a group chat's pending pairing claim (used or cancelled)."""
    with get_db() as conn:
        conn.execute("DELETE FROM pending_pairs WHERE group_chat_id = ?", (group_chat_id,))


# ---------------------------------------------------------------------------
# Partnerships
# ---------------------------------------------------------------------------


def create_partnership(user_a_id: str, user_b_id: str, group_chat_id: str | None = None) -> int:
    """Create a new active partnership between two users. Returns the new partnership id."""
    with get_db() as conn:
        cursor = conn.execute(
            """
            INSERT INTO partnerships (user_a_id, user_b_id, status, group_chat_id)
            VALUES (?, ?, ?, ?)
            """,
            (user_a_id, user_b_id, STATUS_ACTIVE, group_chat_id),
        )
        assert cursor.lastrowid is not None
        return cursor.lastrowid


def get_partnership(partnership_id: int) -> dict[str, Any] | None:
    """Fetch a partnership by id."""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM partnerships WHERE id = ?", (partnership_id,)).fetchone()
        return dict(row) if row else None


def get_active_partnerships_for_user(user_id: str) -> list[dict[str, Any]]:
    """List a user's active partnerships, most recently started first."""
    with get_db() as conn:
        rows = conn.execute(
            """
            SELECT * FROM partnerships
            WHERE (user_a_id = ? OR user_b_id = ?) AND status = ?
            ORDER BY started_at DESC
            """,
            (user_id, user_id, STATUS_ACTIVE),
        ).fetchall()
        return [dict(row) for row in rows]


def get_active_partnership_for_group(group_chat_id: str) -> dict[str, Any] | None:
    """Fetch the active partnership already linked to a group chat, if any."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM partnerships WHERE group_chat_id = ? AND status = ?",
            (group_chat_id, STATUS_ACTIVE),
        ).fetchone()
        return dict(row) if row else None


def get_ended_partnerships_for_user(user_id: str) -> list[dict[str, Any]]:
    """List a user's ended partnerships, most recently ended first."""
    with get_db() as conn:
        rows = conn.execute(
            """
            SELECT * FROM partnerships
            WHERE (user_a_id = ? OR user_b_id = ?) AND status = ?
            ORDER BY ended_at DESC
            """,
            (user_id, user_id, STATUS_ENDED),
        ).fetchall()
        return [dict(row) for row in rows]


def other_user_id(partnership: dict[str, Any], user_id: str) -> str:
    """Given a partnership row and one member's id, return the other member's id."""
    if partnership["user_a_id"] == user_id:
        return str(partnership["user_b_id"])
    return str(partnership["user_a_id"])


def end_partnership(partnership_id: int) -> None:
    """Move a partnership to ended status."""
    with get_db() as conn:
        conn.execute(
            "UPDATE partnerships SET status = ?, ended_at = ? WHERE id = ?",
            (STATUS_ENDED, format_timestamp(utcnow()), partnership_id),
        )


def set_frequency_interval(partnership_id: int, interval_days: int) -> None:
    """Configure a partnership to remind N days after the last logged session."""
    with get_db() as conn:
        conn.execute(
            """
            UPDATE partnerships
            SET frequency_mode = ?, frequency_interval_days = ?,
                frequency_day_of_week = NULL, frequency_time = NULL
            WHERE id = ?
            """,
            (FREQUENCY_INTERVAL, interval_days, partnership_id),
        )


def set_frequency_weekly(partnership_id: int, day_of_week: int, time_str: str) -> None:
    """Configure a partnership to remind on a fixed recurring day/time.

    day_of_week: 0=Monday .. 6=Sunday. time_str: "HH:MM" (24h).
    """
    with get_db() as conn:
        conn.execute(
            """
            UPDATE partnerships
            SET frequency_mode = ?, frequency_day_of_week = ?, frequency_time = ?,
                frequency_interval_days = NULL
            WHERE id = ?
            """,
            (FREQUENCY_WEEKLY, day_of_week, time_str, partnership_id),
        )


def set_group_chat(partnership_id: int, group_chat_id: str) -> None:
    """Link a partnership to a shared group chat for reminders/logging."""
    with get_db() as conn:
        conn.execute(
            "UPDATE partnerships SET group_chat_id = ? WHERE id = ?",
            (group_chat_id, partnership_id),
        )


def get_all_active_partnerships() -> list[dict[str, Any]]:
    """List every active partnership (used to (re)schedule reminders on startup)."""
    with get_db() as conn:
        rows = conn.execute(
            "SELECT * FROM partnerships WHERE status = ?", (STATUS_ACTIVE,)
        ).fetchall()
        return [dict(row) for row in rows]


# ---------------------------------------------------------------------------
# Sessions
# ---------------------------------------------------------------------------


def add_session(partnership_id: int, text_covered: str, logged_by: str) -> int:
    """Log a reading session against a partnership. Returns the new session id."""
    with get_db() as conn:
        cursor = conn.execute(
            "INSERT INTO sessions (partnership_id, text_covered, logged_by) VALUES (?, ?, ?)",
            (partnership_id, text_covered, logged_by),
        )
        assert cursor.lastrowid is not None
        return cursor.lastrowid


def get_sessions_for_partnership(partnership_id: int) -> list[dict[str, Any]]:
    """List all sessions for a partnership, oldest first."""
    with get_db() as conn:
        rows = conn.execute(
            "SELECT * FROM sessions WHERE partnership_id = ? ORDER BY logged_at ASC",
            (partnership_id,),
        ).fetchall()
        return [dict(row) for row in rows]


def get_last_session(partnership_id: int) -> dict[str, Any] | None:
    """Fetch the most recently logged session for a partnership."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM sessions WHERE partnership_id = ? ORDER BY logged_at DESC LIMIT 1",
            (partnership_id,),
        ).fetchone()
        return dict(row) if row else None


def count_sessions_for_partnership(partnership_id: int) -> int:
    """Count sessions logged for a partnership."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT COUNT(*) FROM sessions WHERE partnership_id = ?", (partnership_id,)
        ).fetchone()
        return int(row[0])


# ---------------------------------------------------------------------------
# Stats (for /debug)
# ---------------------------------------------------------------------------


def get_db_stats() -> dict[str, int]:
    """Get database-wide statistics (for debugging)."""
    with get_db() as conn:
        total_users = conn.execute("SELECT COUNT(*) FROM users").fetchone()[0]
        active_partnerships = conn.execute(
            "SELECT COUNT(*) FROM partnerships WHERE status = ?", (STATUS_ACTIVE,)
        ).fetchone()[0]
        ended_partnerships = conn.execute(
            "SELECT COUNT(*) FROM partnerships WHERE status = ?", (STATUS_ENDED,)
        ).fetchone()[0]
        total_sessions = conn.execute("SELECT COUNT(*) FROM sessions").fetchone()[0]

        return {
            "total_users": total_users,
            "active_partnerships": active_partnerships,
            "ended_partnerships": ended_partnerships,
            "total_sessions": total_sessions,
        }
