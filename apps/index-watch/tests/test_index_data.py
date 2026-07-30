"""Tests for index data and historical drawdown frequency."""

from index_watch.index_data import (
    count_trading_days_at_or_below_drawdown,
    historical_drawdown_frequency,
)


def test_count_trading_days_at_or_below_drawdown_empty() -> None:
    assert count_trading_days_at_or_below_drawdown([], -5) == 0


def test_count_trading_days_at_or_below_drawdown_positive_threshold() -> None:
    assert count_trading_days_at_or_below_drawdown([100, 95], 5) == 0


def test_count_trading_days_at_or_below_drawdown_five_pct() -> None:
    # ATH 100; 95 is 5% drawdown. One day at 95.
    closes = [100.0, 95.0, 96.0]
    assert count_trading_days_at_or_below_drawdown(closes, -5) == 1


def test_count_trading_days_at_or_below_drawdown_multiple_days() -> None:
    # 100, 94, 93, 95 -> ATH 100, days at or below 95% of ATH: 94, 93, 95 (3 days)
    closes = [100.0, 94.0, 93.0, 95.0]
    assert count_trading_days_at_or_below_drawdown(closes, -5) == 3


def test_historical_drawdown_frequency() -> None:
    closes = [100.0, 90.0, 85.0, 80.0, 95.0]
    freq = historical_drawdown_frequency(closes, (5, 10, 15, 20))
    assert freq[5] == 4
    assert freq[10] == 3
    assert freq[15] == 2
    assert freq[20] == 1
