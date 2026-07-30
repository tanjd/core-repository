"""Tests for the thread-safe in-memory TTL data cache."""

import threading
from datetime import UTC, datetime, timedelta

from index_watch.cache import CachedData, DataCache


def test_cached_data_is_expired_false_when_fresh() -> None:
    fresh = CachedData(data=1, fetched_at=datetime.now(UTC), ttl_seconds=60)
    assert fresh.is_expired() is False


def test_cached_data_is_expired_true_when_old() -> None:
    old = CachedData(data=1, fetched_at=datetime.now(UTC) - timedelta(seconds=120), ttl_seconds=60)
    assert old.is_expired() is True


def test_set_and_get() -> None:
    cache = DataCache()
    cache.set("key", "value", ttl_seconds=60)
    result = cache.get("key")
    assert result is not None
    data, _fetched_at = result
    assert data == "value"


def test_get_miss() -> None:
    cache = DataCache()
    assert cache.get("missing") is None


def test_get_expired_returns_none_but_keeps_entry_for_stale_fallback() -> None:
    cache = DataCache()
    cache._cache["key"] = CachedData(
        data="stale", fetched_at=datetime.now(UTC) - timedelta(seconds=120), ttl_seconds=60
    )
    assert cache.get("key") is None
    # Entry must survive expiry so get_stale() can still serve it as a
    # graceful-degradation fallback after a failed refetch.
    assert "key" in cache.keys()


def test_get_stale_returns_expired_data() -> None:
    cache = DataCache()
    cache._cache["key"] = CachedData(
        data="stale", fetched_at=datetime.now(UTC) - timedelta(seconds=120), ttl_seconds=60
    )
    result = cache.get_stale("key")
    assert result is not None
    assert result[0] == "stale"


def test_get_stale_missing_key() -> None:
    cache = DataCache()
    assert cache.get_stale("missing") is None


def test_clear() -> None:
    cache = DataCache()
    cache.set("a", 1, 60)
    cache.set("b", 2, 60)
    cache.clear()
    assert cache.keys() == []


def test_get_stats_hit_rate() -> None:
    cache = DataCache()
    cache.set("key", "value", 60)
    cache.get("key")  # hit
    cache.get("missing")  # miss
    stats = cache.get_stats()
    assert stats["hits"] == 1
    assert stats["misses"] == 1
    assert stats["hit_rate_pct"] == 50.0


def test_get_stats_no_requests() -> None:
    cache = DataCache()
    assert cache.get_stats()["hit_rate_pct"] == 0.0


def test_thread_safety_smoke() -> None:
    cache = DataCache()

    def worker(i: int) -> None:
        for _ in range(100):
            cache.set(f"key{i % 5}", i, 60)
            cache.get(f"key{i % 5}")

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(10)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    assert len(cache.keys()) <= 5
