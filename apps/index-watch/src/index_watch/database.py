"""Database operations for subscribers and persistent alert state."""

import logging
import os
import sqlite3
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import datetime
from pathlib import Path
from typing import Any, TypedDict


class SubscriberOverride(TypedDict, total=False):
    """A subscriber's threshold/index overrides; keys are absent when using the default."""

    thresholds: tuple[int, ...]
    indices: tuple[str, ...]


logger = logging.getLogger(__name__)

# Database file location. Defaults to the app's own data/ dir (inside Docker:
# /app/apps/index-watch/data/); override with the DB_PATH env var, e.g. to point at a
# volume mounted at a different path. Config.from_env() parses DB_PATH too (for logging
# purposes), but this is the module that actually opens the file, so it must read the
# same env var itself rather than trusting a value handed to it.
DB_PATH = Path(
    os.getenv("DB_PATH", str(Path(__file__).parent.parent.parent / "data" / "index_watch.db"))
)


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

        # One-time ad-hoc migration: alert_state's primary key gained chat_id
        # (per-subscriber alert state). CREATE TABLE IF NOT EXISTS won't alter an
        # existing table, so detect the old schema and drop it - this loses
        # in-flight "already alerted" state (one possible duplicate alert per
        # already-triggered threshold), which is accepted as a one-time,
        # low-severity side effect of the upgrade.
        alert_state_cols = {row["name"] for row in conn.execute("PRAGMA table_info(alert_state)")}
        if alert_state_cols and "chat_id" not in alert_state_cols:
            logger.warning(
                "Migrating alert_state to per-subscriber schema (dropping old table; "
                "may cause one duplicate alert per already-triggered threshold)"
            )
            conn.execute("DROP TABLE alert_state")

        # Alert state persistence table (per-subscriber)
        conn.execute("""
            CREATE TABLE IF NOT EXISTS alert_state (
                chat_id TEXT,
                symbol TEXT,
                threshold_pct INTEGER,
                sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (chat_id, symbol, threshold_pct)
            )
        """)

        # Recovery/new-ATH notification state persistence table (per-subscriber)
        conn.execute("""
            CREATE TABLE IF NOT EXISTS recovery_state (
                chat_id TEXT,
                symbol TEXT,
                notified_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (chat_id, symbol)
            )
        """)

        # Per-subscriber overrides: absence of rows for a chat_id means "use
        # the global config defaults" - no backfill needed for existing subscribers.
        conn.execute("""
            CREATE TABLE IF NOT EXISTS subscriber_thresholds (
                chat_id TEXT NOT NULL,
                threshold_pct INTEGER NOT NULL,
                PRIMARY KEY (chat_id, threshold_pct)
            )
        """)

        conn.execute("""
            CREATE TABLE IF NOT EXISTS subscriber_indices (
                chat_id TEXT NOT NULL,
                symbol TEXT NOT NULL,
                PRIMARY KEY (chat_id, symbol)
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


def load_alert_state() -> set[tuple[str, str, int]]:
    """Load persistent per-subscriber alert state from database."""
    with get_db() as conn:
        rows = conn.execute("SELECT chat_id, symbol, threshold_pct FROM alert_state").fetchall()
        result = {(row["chat_id"], row["symbol"], row["threshold_pct"]) for row in rows}
        logger.info("Loaded %d alert states from database", len(result))
        return result


def save_alert_state(state: set[tuple[str, str, int]]) -> None:
    """Persist per-subscriber alert state to database."""
    with get_db() as conn:
        # Clear existing state
        conn.execute("DELETE FROM alert_state")
        # Insert current state
        for chat_id, symbol, threshold_pct in state:
            conn.execute(
                "INSERT INTO alert_state (chat_id, symbol, threshold_pct) VALUES (?, ?, ?)",
                (chat_id, symbol, threshold_pct),
            )
        logger.info("Saved %d alert states to database", len(state))


def clear_alert_state() -> None:
    """Clear all alert state (for testing or manual reset)."""
    with get_db() as conn:
        conn.execute("DELETE FROM alert_state")
    logger.info("Cleared all alert state from database")


def load_recovery_state() -> set[tuple[str, str]]:
    """Load persistent per-subscriber recovery/new-ATH notification state from database."""
    with get_db() as conn:
        rows = conn.execute("SELECT chat_id, symbol FROM recovery_state").fetchall()
        result = {(row["chat_id"], row["symbol"]) for row in rows}
        logger.info("Loaded %d recovery states from database", len(result))
        return result


def save_recovery_state(state: set[tuple[str, str]]) -> None:
    """Persist per-subscriber recovery/new-ATH notification state to database."""
    with get_db() as conn:
        conn.execute("DELETE FROM recovery_state")
        for chat_id, symbol in state:
            conn.execute(
                "INSERT INTO recovery_state (chat_id, symbol) VALUES (?, ?)",
                (chat_id, symbol),
            )
        logger.info("Saved %d recovery states to database", len(state))


def clear_recovery_state() -> None:
    """Clear all recovery state (for testing or manual reset)."""
    with get_db() as conn:
        conn.execute("DELETE FROM recovery_state")
    logger.info("Cleared all recovery state from database")


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


def get_subscriber_thresholds(chat_id: str) -> tuple[int, ...] | None:
    """Get a subscriber's threshold override, or None if using the global default."""
    with get_db() as conn:
        rows = conn.execute(
            "SELECT threshold_pct FROM subscriber_thresholds WHERE chat_id = ? "
            "ORDER BY threshold_pct",
            (chat_id,),
        ).fetchall()
        if not rows:
            return None
        return tuple(row["threshold_pct"] for row in rows)


def set_subscriber_thresholds(chat_id: str, thresholds: list[int]) -> None:
    """Set a subscriber's threshold override (wholesale replace)."""
    with get_db() as conn:
        conn.execute("DELETE FROM subscriber_thresholds WHERE chat_id = ?", (chat_id,))
        for threshold_pct in thresholds:
            conn.execute(
                "INSERT INTO subscriber_thresholds (chat_id, threshold_pct) VALUES (?, ?)",
                (chat_id, threshold_pct),
            )


def clear_subscriber_thresholds(chat_id: str) -> None:
    """Clear a subscriber's threshold override, reverting them to the global default."""
    with get_db() as conn:
        conn.execute("DELETE FROM subscriber_thresholds WHERE chat_id = ?", (chat_id,))


def get_subscriber_indices(chat_id: str) -> tuple[str, ...] | None:
    """Get a subscriber's index override, or None if using the global default."""
    with get_db() as conn:
        rows = conn.execute(
            "SELECT symbol FROM subscriber_indices WHERE chat_id = ? ORDER BY symbol",
            (chat_id,),
        ).fetchall()
        if not rows:
            return None
        return tuple(row["symbol"] for row in rows)


def set_subscriber_indices(chat_id: str, symbols: list[str]) -> None:
    """Set a subscriber's index override (wholesale replace)."""
    with get_db() as conn:
        conn.execute("DELETE FROM subscriber_indices WHERE chat_id = ?", (chat_id,))
        for symbol in symbols:
            conn.execute(
                "INSERT INTO subscriber_indices (chat_id, symbol) VALUES (?, ?)",
                (chat_id, symbol),
            )


def clear_subscriber_indices(chat_id: str) -> None:
    """Clear a subscriber's index override, reverting them to the global default."""
    with get_db() as conn:
        conn.execute("DELETE FROM subscriber_indices WHERE chat_id = ?", (chat_id,))


def get_all_subscriber_overrides() -> dict[str, SubscriberOverride]:
    """
    Bulk-load every subscriber's overrides in two queries (for the alert-check loop).

    Returns:
        {chat_id: {"thresholds": (...), "indices": (...)}} - a chat_id only appears
        under a key if it has an override for that setting.
    """
    overrides: dict[str, SubscriberOverride] = {}
    with get_db() as conn:
        threshold_rows = conn.execute(
            "SELECT chat_id, threshold_pct FROM subscriber_thresholds "
            "ORDER BY chat_id, threshold_pct"
        ).fetchall()
        index_rows = conn.execute(
            "SELECT chat_id, symbol FROM subscriber_indices ORDER BY chat_id, symbol"
        ).fetchall()

    thresholds_by_chat: dict[str, list[int]] = {}
    for row in threshold_rows:
        thresholds_by_chat.setdefault(row["chat_id"], []).append(row["threshold_pct"])
    for chat_id, thresholds in thresholds_by_chat.items():
        overrides.setdefault(chat_id, {})["thresholds"] = tuple(thresholds)

    indices_by_chat: dict[str, list[str]] = {}
    for row in index_rows:
        indices_by_chat.setdefault(row["chat_id"], []).append(row["symbol"])
    for chat_id, symbols in indices_by_chat.items():
        overrides.setdefault(chat_id, {})["indices"] = tuple(symbols)

    return overrides


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
