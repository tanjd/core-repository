"""Simple in-memory cache with TTL for market data."""

import logging
import threading
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

logger = logging.getLogger(__name__)


@dataclass
class CachedData[T]:
    """Cached data with timestamp and TTL."""

    data: T
    fetched_at: datetime
    ttl_seconds: int

    def is_expired(self) -> bool:
        """Check if cached data has expired."""
        age = datetime.now(UTC) - self.fetched_at
        return age.total_seconds() > self.ttl_seconds


class DataCache:
    """Thread-safe in-memory cache with TTL support."""

    def __init__(self):
        self._cache: dict[str, CachedData] = {}
        self._lock = threading.Lock()
        self._stats = {"hits": 0, "misses": 0, "expirations": 0}
        logger.info("Data cache initialized")

    def get(self, key: str) -> tuple[Any, datetime] | None:
        """
        Get cached data if not expired.

        Returns:
            tuple of (data, fetched_at) if valid cache exists, None otherwise
        """
        with self._lock:
            cached = self._cache.get(key)
            if cached and not cached.is_expired():
                self._stats["hits"] += 1
                age = (datetime.now(UTC) - cached.fetched_at).total_seconds()
                logger.debug("Cache HIT: key=%s age=%.1fs", key, age)
                return cached.data, cached.fetched_at
            # Leave the expired entry in place (not deleted) so get_stale() can
            # still serve it as a graceful-degradation fallback on fetch failure.
            if cached:
                self._stats["expirations"] += 1
                logger.debug("Cache EXPIRED: key=%s", key)
            else:
                logger.debug("Cache MISS: key=%s", key)
            self._stats["misses"] += 1
            return None

    def get_stale(self, key: str) -> tuple[Any, datetime] | None:
        """
        Get cached data even if expired (for graceful degradation).

        Use this as a fallback when fresh data fetching fails.

        Returns:
            tuple of (data, fetched_at) if any cache exists, None otherwise
        """
        with self._lock:
            cached = self._cache.get(key)
            if cached:
                age = (datetime.now(UTC) - cached.fetched_at).total_seconds()
                logger.warning(
                    "Serving STALE cache: key=%s age=%.1fs (TTL=%ds)",
                    key,
                    age,
                    cached.ttl_seconds,
                )
                return cached.data, cached.fetched_at
            return None

    def set(self, key: str, data: Any, ttl_seconds: int) -> None:
        """Store data with TTL in cache."""
        with self._lock:
            self._cache[key] = CachedData(
                data=data, fetched_at=datetime.now(UTC), ttl_seconds=ttl_seconds
            )
            logger.debug("Cache SET: key=%s ttl=%ds", key, ttl_seconds)

    def clear(self) -> None:
        """Clear all cached data."""
        with self._lock:
            count = len(self._cache)
            self._cache.clear()
            self._stats = {"hits": 0, "misses": 0, "expirations": 0}
            logger.info("Cache cleared: removed %d entries", count)

    def keys(self) -> list[str]:
        """Get all cache keys (for debugging)."""
        with self._lock:
            return list(self._cache.keys())

    def get_stats(self) -> dict[str, Any]:
        """Get cache statistics."""
        with self._lock:
            total_requests = self._stats["hits"] + self._stats["misses"]
            hit_rate = self._stats["hits"] / total_requests * 100 if total_requests > 0 else 0.0
            return {
                "entries": len(self._cache),
                "hits": self._stats["hits"],
                "misses": self._stats["misses"],
                "expirations": self._stats["expirations"],
                "hit_rate_pct": round(hit_rate, 1),
            }


# Global cache instance
_global_cache = DataCache()


def get_cache() -> DataCache:
    """Get the global cache instance."""
    return _global_cache
