"""Tests for database persistence (subscribers + alert state)."""

import sqlite3

from index_watch import database


def test_add_subscriber_new() -> None:
    assert database.add_subscriber("123", username="alice") is True
    assert database.is_subscribed("123") is True


def test_add_subscriber_already_active() -> None:
    database.add_subscriber("123")
    assert database.add_subscriber("123") is False


def test_add_subscriber_reactivate() -> None:
    database.add_subscriber("123")
    database.remove_subscriber("123")
    assert database.is_subscribed("123") is False
    assert database.add_subscriber("123") is True
    assert database.is_subscribed("123") is True


def test_remove_subscriber_not_found() -> None:
    assert database.remove_subscriber("999") is False


def test_remove_subscriber_already_inactive() -> None:
    database.add_subscriber("123")
    database.remove_subscriber("123")
    assert database.remove_subscriber("123") is False


def test_get_active_subscribers() -> None:
    database.add_subscriber("1")
    database.add_subscriber("2")
    database.remove_subscriber("2")
    assert database.get_active_subscribers() == ["1"]


def test_is_subscribed_unknown_user() -> None:
    assert database.is_subscribed("nope") is False


def test_get_subscriber_stats_not_found() -> None:
    assert database.get_subscriber_stats("nope") is None


def test_get_subscriber_stats_found() -> None:
    database.add_subscriber("123", username="alice")
    stats = database.get_subscriber_stats("123")
    assert stats is not None
    assert stats["active"] is True
    assert stats["last_daily_sent"] is None
    assert stats["last_alert_sent"] is None


def test_update_last_daily_sent() -> None:
    database.add_subscriber("123")
    database.update_last_daily_sent("123")
    stats = database.get_subscriber_stats("123")
    assert stats is not None
    assert stats["last_daily_sent"] is not None


def test_update_last_alert_sent() -> None:
    database.add_subscriber("123")
    database.update_last_alert_sent("123")
    stats = database.get_subscriber_stats("123")
    assert stats is not None
    assert stats["last_alert_sent"] is not None


def test_load_alert_state_empty() -> None:
    assert database.load_alert_state() == set()


def test_save_and_load_alert_state() -> None:
    state = {("111", "^GSPC", 5), ("222", "^NDX", 10)}
    database.save_alert_state(state)
    assert database.load_alert_state() == state


def test_save_alert_state_overwrites() -> None:
    database.save_alert_state({("111", "^GSPC", 5)})
    database.save_alert_state({("111", "^NDX", 10)})
    assert database.load_alert_state() == {("111", "^NDX", 10)}


def test_clear_alert_state() -> None:
    database.save_alert_state({("111", "^GSPC", 5)})
    database.clear_alert_state()
    assert database.load_alert_state() == set()


def test_migrate_env_chat_ids() -> None:
    count = database.migrate_env_chat_ids(["1", "2", "3"])
    assert count == 3
    assert set(database.get_active_subscribers()) == {"1", "2", "3"}


def test_migrate_env_chat_ids_idempotent() -> None:
    database.migrate_env_chat_ids(["1", "2"])
    count = database.migrate_env_chat_ids(["1", "2"])
    assert count == 0


def test_migrate_env_chat_ids_empty() -> None:
    assert database.migrate_env_chat_ids([]) == 0


def test_get_db_stats() -> None:
    database.add_subscriber("1")
    database.add_subscriber("2")
    database.remove_subscriber("2")
    database.save_alert_state({("111", "^GSPC", 5)})
    stats = database.get_db_stats()
    assert stats == {"total_subscribers": 2, "active_subscribers": 1, "alert_states": 1}


# -- Per-subscriber threshold/index overrides --------------------------------


def test_get_subscriber_thresholds_none_by_default() -> None:
    assert database.get_subscriber_thresholds("123") is None


def test_set_and_get_subscriber_thresholds() -> None:
    database.set_subscriber_thresholds("123", [10, 5, 20])
    assert database.get_subscriber_thresholds("123") == (5, 10, 20)


def test_set_subscriber_thresholds_overwrites() -> None:
    database.set_subscriber_thresholds("123", [5, 10])
    database.set_subscriber_thresholds("123", [15])
    assert database.get_subscriber_thresholds("123") == (15,)


def test_clear_subscriber_thresholds_reverts_to_default() -> None:
    database.set_subscriber_thresholds("123", [5, 10])
    database.clear_subscriber_thresholds("123")
    assert database.get_subscriber_thresholds("123") is None


def test_subscriber_thresholds_isolated_per_chat_id() -> None:
    database.set_subscriber_thresholds("123", [5])
    assert database.get_subscriber_thresholds("456") is None


def test_get_subscriber_indices_none_by_default() -> None:
    assert database.get_subscriber_indices("123") is None


def test_set_and_get_subscriber_indices() -> None:
    database.set_subscriber_indices("123", ["^NDX", "^GSPC"])
    assert database.get_subscriber_indices("123") == ("^GSPC", "^NDX")


def test_set_subscriber_indices_overwrites() -> None:
    database.set_subscriber_indices("123", ["^GSPC", "^NDX"])
    database.set_subscriber_indices("123", ["^NDX"])
    assert database.get_subscriber_indices("123") == ("^NDX",)


def test_clear_subscriber_indices_reverts_to_default() -> None:
    database.set_subscriber_indices("123", ["^GSPC"])
    database.clear_subscriber_indices("123")
    assert database.get_subscriber_indices("123") is None


def test_get_all_subscriber_overrides_empty() -> None:
    assert database.get_all_subscriber_overrides() == {}


def test_get_all_subscriber_overrides_combines_thresholds_and_indices() -> None:
    database.set_subscriber_thresholds("123", [5, 10])
    database.set_subscriber_indices("123", ["^GSPC"])
    database.set_subscriber_thresholds("456", [20])

    overrides = database.get_all_subscriber_overrides()

    assert overrides["123"].get("thresholds") == (5, 10)
    assert overrides["123"].get("indices") == ("^GSPC",)
    assert overrides["456"].get("thresholds") == (20,)
    assert "indices" not in overrides["456"]


def test_init_db_migrates_old_alert_state_schema() -> None:
    """A pre-migration alert_state table (no chat_id column) must be upgraded in place."""
    conn = sqlite3.connect(str(database.DB_PATH))
    try:
        conn.execute("DROP TABLE IF EXISTS alert_state")
        conn.execute("""
            CREATE TABLE alert_state (
                symbol TEXT,
                threshold_pct INTEGER,
                sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (symbol, threshold_pct)
            )
        """)
        conn.execute("INSERT INTO alert_state (symbol, threshold_pct) VALUES ('^GSPC', 5)")
        conn.commit()
    finally:
        conn.close()

    database.init_db()

    cols = set()
    conn = sqlite3.connect(str(database.DB_PATH))
    try:
        cols = {row[1] for row in conn.execute("PRAGMA table_info(alert_state)")}
    finally:
        conn.close()

    assert "chat_id" in cols
    # Old pre-migration row must be gone (accepted lossy migration).
    assert database.load_alert_state() == set()


# -- Recovery/new-ATH notification state --------------------------------------


def test_load_recovery_state_empty() -> None:
    assert database.load_recovery_state() == set()


def test_save_and_load_recovery_state() -> None:
    state = {("111", "^GSPC"), ("222", "^NDX")}
    database.save_recovery_state(state)
    assert database.load_recovery_state() == state


def test_save_recovery_state_overwrites() -> None:
    database.save_recovery_state({("111", "^GSPC")})
    database.save_recovery_state({("111", "^NDX")})
    assert database.load_recovery_state() == {("111", "^NDX")}


def test_clear_recovery_state() -> None:
    database.save_recovery_state({("111", "^GSPC")})
    database.clear_recovery_state()
    assert database.load_recovery_state() == set()
