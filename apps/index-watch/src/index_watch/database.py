"""Database operations for subscribers and persistent alert state."""

import logging
import sqlite3
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import datetime
from pathlib import Path
from typing import Any

logger = logging.getLogger(__name__)

# Database file location (inside Docker container: /app/data/)
DB_PATH = Path(__file__).parent.parent.parent / "data" / "index_watch.db"


@contextmanager
def get_db() -> Iterator[sqlite3.Connection]:
    """Context manager for database connection with auto-commit/rollback."""
    conn = sqlite3.connect(str(DB_PATH))
    conn.row_factory = sqlite3.Row
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
    # Ensure data directory exists
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)

    with get_db() as conn:
        # Subscribers table
        conn.execute("""
            CREATE TABLE IF NOT EXISTS subscribers (
                chat_id TEXT PRIMARY KEY,
                username TEXT,
                subscribed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                last_daily_sent TIMESTAMP,
                last_alert_sent TIMESTAMP,
                active INTEGER DEFAULT 1
            )
        """)

        # Index for active subscribers query
        conn.execute("""
            CREATE INDEX IF NOT EXISTS idx_subscribers_active
            ON subscribers(active)
        """)

        # Alert state persistence table
        conn.execute("""
            CREATE TABLE IF NOT EXISTS alert_state (
                symbol TEXT,
                threshold_pct INTEGER,
                sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (symbol, threshold_pct)
            )
        """)

    logger.info("Database initialized at %s", DB_PATH)


def add_subscriber(chat_id: str, username: str | None = None) -> bool:
    """
    Subscribe a user to notifications.

    Returns:
        True if newly added, False if already subscribed
    """
    with get_db() as conn:
        # Check if already exists and active
        existing = conn.execute(
            "SELECT active FROM subscribers WHERE chat_id = ?", (chat_id,)
        ).fetchone()

        if existing:
            if existing["active"] == 1:
                logger.info("User %s already subscribed", chat_id)
                return False
            # Reactivate previously unsubscribed user
            conn.execute(
                "UPDATE subscribers SET active = 1, subscribed_at = ? WHERE chat_id = ?",
                (datetime.now(), chat_id),
            )
            logger.info("Reactivated subscription for %s", chat_id)
            return True

        # Insert new subscriber
        conn.execute(
            "INSERT INTO subscribers (chat_id, username) VALUES (?, ?)", (chat_id, username)
        )
        logger.info("Added new subscriber: %s (username: %s)", chat_id, username)
        return True


def remove_subscriber(chat_id: str) -> bool:
    """
    Unsubscribe a user (soft delete).

    Returns:
        True if unsubscribed, False if not found or already inactive
    """
    with get_db() as conn:
        result = conn.execute(
            "UPDATE subscribers SET active = 0 WHERE chat_id = ? AND active = 1",
            (chat_id,),
        )
        if result.rowcount > 0:
            logger.info("Unsubscribed user: %s", chat_id)
            return True
        logger.warning("User %s not found or already unsubscribed", chat_id)
        return False


def get_active_subscribers() -> list[str]:
    """Get all active subscriber chat IDs."""
    with get_db() as conn:
        rows = conn.execute("SELECT chat_id FROM subscribers WHERE active = 1").fetchall()
        return [row["chat_id"] for row in rows]


def is_subscribed(chat_id: str) -> bool:
    """Check if a user is subscribed."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT active FROM subscribers WHERE chat_id = ?", (chat_id,)
        ).fetchone()
        return row is not None and row["active"] == 1


def get_subscriber_stats(chat_id: str) -> dict[str, Any] | None:
    """Get subscription stats for a user."""
    with get_db() as conn:
        row = conn.execute(
            """
            SELECT
                subscribed_at,
                last_daily_sent,
                last_alert_sent,
                active
            FROM subscribers
            WHERE chat_id = ?
            """,
            (chat_id,),
        ).fetchone()

        if not row:
            return None

        return {
            "subscribed_at": row["subscribed_at"],
            "last_daily_sent": row["last_daily_sent"],
            "last_alert_sent": row["last_alert_sent"],
            "active": bool(row["active"]),
        }


def update_last_daily_sent(chat_id: str) -> None:
    """Update timestamp of last daily report sent."""
    with get_db() as conn:
        conn.execute(
            "UPDATE subscribers SET last_daily_sent = ? WHERE chat_id = ?",
            (datetime.now(), chat_id),
        )


def update_last_alert_sent(chat_id: str) -> None:
    """Update timestamp of last alert sent."""
    with get_db() as conn:
        conn.execute(
            "UPDATE subscribers SET last_alert_sent = ? WHERE chat_id = ?",
            (datetime.now(), chat_id),
        )


def load_alert_state() -> set[tuple[str, int]]:
    """Load persistent alert state from database."""
    with get_db() as conn:
        rows = conn.execute("SELECT symbol, threshold_pct FROM alert_state").fetchall()
        result = {(row["symbol"], row["threshold_pct"]) for row in rows}
        logger.info("Loaded %d alert states from database", len(result))
        return result


def save_alert_state(state: set[tuple[str, int]]) -> None:
    """Persist alert state to database."""
    with get_db() as conn:
        # Clear existing state
        conn.execute("DELETE FROM alert_state")
        # Insert current state
        for symbol, threshold_pct in state:
            conn.execute(
                "INSERT INTO alert_state (symbol, threshold_pct) VALUES (?, ?)",
                (symbol, threshold_pct),
            )
        logger.info("Saved %d alert states to database", len(state))


def clear_alert_state() -> None:
    """Clear all alert state (for testing or manual reset)."""
    with get_db() as conn:
        conn.execute("DELETE FROM alert_state")
    logger.info("Cleared all alert state from database")


def migrate_env_chat_ids(chat_ids: list[str]) -> int:
    """
    Migrate chat IDs from .env to database (one-time migration).

    Returns:
        Number of chat IDs migrated
    """
    if not chat_ids:
        return 0

    migrated_count = 0
    for chat_id in chat_ids:
        if add_subscriber(chat_id, username=None):
            migrated_count += 1

    if migrated_count > 0:
        logger.info("Migrated %d chat IDs from .env to database", migrated_count)
    return migrated_count


def get_db_stats() -> dict[str, int]:
    """Get database statistics (for debugging)."""
    with get_db() as conn:
        total_subscribers = conn.execute("SELECT COUNT(*) FROM subscribers").fetchone()[0]
        active_subscribers = conn.execute(
            "SELECT COUNT(*) FROM subscribers WHERE active = 1"
        ).fetchone()[0]
        alert_states = conn.execute("SELECT COUNT(*) FROM alert_state").fetchone()[0]

        return {
            "total_subscribers": total_subscribers,
            "active_subscribers": active_subscribers,
            "alert_states": alert_states,
        }
