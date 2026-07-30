"""Fetch index prices and compute metrics using yfinance."""

import logging
from datetime import UTC, datetime, timedelta

import yfinance as yf

from index_watch.cache import get_cache
from index_watch.drawdown import (
    DrawdownMetrics,
    compute_ath_and_lowest_since_ath,
    compute_drawdown_metrics,
)

logger = logging.getLogger(__name__)

# Cache TTL: 30 minutes for index data (matches alert check interval)
CACHE_TTL_SECONDS = 30 * 60


def fetch_index_history(symbol: str, years: int = 20) -> tuple[list[float], datetime, bool]:
    """
    Fetch historical daily close prices (oldest first) with caching and graceful degradation.

    Returns:
        tuple of (closes, fetched_at, is_stale) - closes is empty list on complete failure,
        fetched_at is when data was retrieved (UTC), is_stale indicates if serving old cache
    """
    cache = get_cache()
    cache_key = f"index_history:{symbol}:{years}"

    # Check cache first
    cached = cache.get(cache_key)
    if cached:
        closes, fetched_at = cached
        logger.info(
            "Using cached data for %s (%d days, cached %d seconds ago)",
            symbol,
            len(closes),
            int((datetime.now(UTC) - fetched_at).total_seconds()),
        )
        return closes, fetched_at, False

    # Fetch fresh data
    end = datetime.now(UTC)
    start = end - timedelta(days=years * 365)
    fetched_at = datetime.now(UTC)

    try:
        ticker = yf.Ticker(symbol)
        hist = ticker.history(start=start, end=end, auto_adjust=True)
        if hist is None or hist.empty:
            logger.warning(
                "No data returned for %s (start=%s, end=%s) - trying stale cache",
                symbol,
                start.date(),
                end.date(),
            )
            # Try stale cache as fallback
            stale = cache.get_stale(cache_key)
            if stale:
                closes, fetched_at = stale
                logger.info("Using stale cache for %s (%d days)", symbol, len(closes))
                return closes, fetched_at, True
            return [], fetched_at, False

        closes = hist["Close"].dropna().tolist()
        result = [float(c) for c in closes]
        logger.info("Fetched %d days of history for %s", len(result), symbol)

        # Cache the result
        cache.set(cache_key, result, CACHE_TTL_SECONDS)
        return result, fetched_at, False

    except Exception as e:
        logger.error("Failed to fetch data for %s: %s - trying stale cache", symbol, e)
        # Try stale cache as fallback
        stale = cache.get_stale(cache_key)
        if stale:
            closes, fetched_at = stale
            logger.info("Using stale cache for %s (%d days) after error", symbol, len(closes))
            return closes, fetched_at, True
        return [], fetched_at, False


def get_index_metrics(
    symbol: str, display_name: str, years: int = 20
) -> tuple[DrawdownMetrics, datetime, bool] | None:
    """
    Get drawdown metrics for an index with data timestamp.

    Returns:
        tuple of (metrics, fetched_at, is_stale) or None if data unavailable
        is_stale=True means data is from expired cache (API failure fallback)
    """
    closes, fetched_at, is_stale = fetch_index_history(symbol, years=years)
    if len(closes) < 2:
        return None
    current_price = closes[-1]
    ath, lowest_since_ath = compute_ath_and_lowest_since_ath(closes)
    metrics = compute_drawdown_metrics(current_price, ath, lowest_since_ath)
    return metrics, fetched_at, is_stale


def count_trading_days_at_or_below_drawdown(closes: list[float], threshold_pct: float) -> int:
    """Count how many trading days the index closed at or below this drawdown from its then-ATH."""
    if not closes or threshold_pct >= 0:
        return 0
    # threshold_pct is e.g. -5 for "5% drawdown"
    threshold_ratio = 1 + (threshold_pct / 100)
    count = 0
    ath = closes[0]
    for p in closes:
        if p > ath:
            ath = p
        if ath > 0 and p / ath <= threshold_ratio:
            count += 1
    return count


def historical_drawdown_frequency(
    closes: list[float], thresholds_pct: tuple[int, ...]
) -> dict[int, int]:
    """Days at or below each drawdown threshold (e.g. 5, 10, 15, 20)."""
    return {t: count_trading_days_at_or_below_drawdown(closes, -t) for t in thresholds_pct}
