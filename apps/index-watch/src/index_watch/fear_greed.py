"""CNN Fear & Greed Index fetcher."""

import logging
from dataclasses import dataclass
from datetime import UTC, datetime

from index_watch.cache import get_cache

logger = logging.getLogger(__name__)

# Cache TTL: 30 minutes (Fear & Greed updates once daily, but we match index data TTL)
CACHE_TTL_SECONDS = 30 * 60


@dataclass
class FearGreedResult:
    """Fear and Greed index result."""

    value: float
    description: str
    last_update: str  # human-readable or ISO


def fetch_fear_greed() -> FearGreedResult | None:
    """
    Fetch current CNN Fear & Greed Index with caching. Returns None on failure.

    Cached for 30 minutes to reduce API calls.
    """
    cache = get_cache()
    cache_key = "fear_greed:latest"

    # Check cache first
    cached = cache.get(cache_key)
    if cached:
        result, fetched_at = cached

        age = int((datetime.now(UTC) - fetched_at).total_seconds())
        logger.info(
            "Using cached Fear & Greed: %.1f (%s) - cached %ds ago",
            result.value,
            result.description,
            age,
        )
        return result

    # Fetch fresh data
    try:
        import fear_and_greed

        fg = fear_and_greed.get()
        result = FearGreedResult(
            value=fg.value,
            description=fg.description or "Unknown",
            last_update=(
                fg.last_update.isoformat()
                if getattr(fg.last_update, "isoformat", None)
                else str(fg.last_update)
            ),
        )
        logger.info("Fetched Fear & Greed: %.1f (%s)", result.value, result.description)

        # Cache the result
        cache.set(cache_key, result, CACHE_TTL_SECONDS)
        return result
    except Exception as e:
        logger.warning("Failed to fetch Fear & Greed Index: %s - trying stale cache", e)
        # Try stale cache as fallback
        stale = cache.get_stale(cache_key)
        if stale:
            result, _ = stale
            logger.info(
                "Using stale Fear & Greed cache: %.1f (%s)", result.value, result.description
            )
            return result
        return None
