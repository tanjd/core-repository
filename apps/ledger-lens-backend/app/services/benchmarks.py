"""Fetch + cache benchmark index price history from Yahoo Finance's chart API.

Historical closes never change, so `BenchmarkPrice` rows are a permanent cache keyed on
(symbol, price_date) — once a date is cached it's never re-fetched. A fetch failure (network
error, rate limit, malformed response) is logged and swallowed rather than raised, so a transient
upstream outage degrades to "serve whatever is already cached" instead of breaking the comparison
chart.

Stooq (the original candidate) was ruled out — as of this writing it puts a JavaScript
proof-of-work challenge in front of every request, including its plain CSV export, so a
server-side HTTP client can't fetch data from it at all.
"""

from __future__ import annotations

import logging
from datetime import UTC, date, datetime
from typing import Any

import httpx
from sqlmodel import Session, select

from app.models.db import BenchmarkPrice

logger = logging.getLogger(__name__)

_CHART_URL = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}"
_TIMEOUT_SECONDS = 10.0
# Yahoo's endpoint 404s or rate-limits requests with no browser-like User-Agent.
_HEADERS = {"User-Agent": "Mozilla/5.0"}

# Friendly symbol -> (Yahoo ticker, display label). Keep this to a handful of common,
# broadly-relevant indices/ETFs rather than accepting arbitrary tickers — users pick from a
# dropdown instead of needing to know exchange-suffix ticker syntax (e.g. "VWRA.L").
BENCHMARK_CATALOG: dict[str, tuple[str, str]] = {
    "SPY": ("SPY", "S&P 500 (SPY)"),
    "QQQ": ("QQQ", "Nasdaq 100 (QQQ)"),
    "VTI": ("VTI", "Total US Market (VTI)"),
    "VWRA": ("VWRA.L", "FTSE All-World (VWRA)"),
}


def _fetch_chart_json(yahoo_symbol: str) -> dict[str, Any]:
    resp = httpx.get(
        _CHART_URL.format(symbol=yahoo_symbol),
        params={"range": "max", "interval": "1d"},
        headers=_HEADERS,
        timeout=_TIMEOUT_SECONDS,
    )
    resp.raise_for_status()
    return resp.json()


def _parse_chart_json(payload: dict[str, Any]) -> list[tuple[date, float]]:
    result = (payload.get("chart") or {}).get("result") or []
    if not result:
        return []
    timestamps = result[0].get("timestamp") or []
    closes = ((result[0].get("indicators") or {}).get("quote") or [{}])[0].get("close") or []

    rows: list[tuple[date, float]] = []
    for ts, close in zip(timestamps, closes, strict=False):
        if ts is None or close is None:
            continue
        rows.append((datetime.fromtimestamp(ts, tz=UTC).date(), float(close)))
    return rows


def ensure_benchmark_prices(symbol: str, session: Session, start: date, end: date) -> None:
    """Populate the BenchmarkPrice cache for `symbol` over [start, end] if not already covered."""
    catalog_entry = BENCHMARK_CATALOG.get(symbol)
    if catalog_entry is None:
        return
    yahoo_symbol, _label = catalog_entry

    cached_dates = set(
        session.exec(select(BenchmarkPrice.price_date).where(BenchmarkPrice.symbol == symbol)).all()
    )
    covers_range = bool(cached_dates) and min(cached_dates) <= start and max(cached_dates) >= end
    if covers_range:
        return

    try:
        payload = _fetch_chart_json(yahoo_symbol)
        rows = _parse_chart_json(payload)
    except Exception:
        logger.exception("Failed to fetch benchmark prices for %s from Yahoo Finance", symbol)
        return

    for price_date, close in rows:
        if price_date in cached_dates:
            continue
        session.add(BenchmarkPrice(symbol=symbol, price_date=price_date, close=close))
        cached_dates.add(price_date)
    session.commit()
