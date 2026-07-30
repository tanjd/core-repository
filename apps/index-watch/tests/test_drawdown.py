"""Tests for drawdown metrics calculation."""

import pytest

from index_watch.drawdown import (
    compute_ath_and_lowest_since_ath,
    compute_drawdown_metrics,
)


def test_compute_drawdown_metrics_basic() -> None:
    """Current price below ATH yields negative drawdown."""
    m = compute_drawdown_metrics(
        current_price=6836.17,
        ath=7002.28,
        lowest_since_ath=6780.13,
    )
    assert m.current_price == 6836.17
    assert m.ath == 7002.28
    assert m.lowest_since_ath == 6780.13
    assert m.current_drawdown_pct == pytest.approx(-2.37, abs=0.02)
    assert m.drawdown_at_lowest_pct == pytest.approx(-3.17, abs=0.02)
    assert m.gain_from_lowest_pct == pytest.approx(0.83, abs=0.02)
    assert m.gain_to_ath_from_current_pct == pytest.approx(2.43, abs=0.02)
    assert m.gain_to_ath_from_lowest_pct == pytest.approx(3.28, abs=0.02)


def test_compute_drawdown_metrics_at_ath() -> None:
    """At ATH drawdown is 0, gains from lowest are positive."""
    m = compute_drawdown_metrics(current_price=100.0, ath=100.0, lowest_since_ath=80.0)
    assert m.current_drawdown_pct == 0.0
    assert m.gain_from_lowest_pct == 25.0
    assert m.gain_to_ath_from_current_pct == 0.0


def test_compute_drawdown_metrics_ath_zero_raises() -> None:
    with pytest.raises(ValueError, match="ATH must be positive"):
        compute_drawdown_metrics(100.0, 0.0, 90.0)


def test_compute_ath_and_lowest_since_ath_empty() -> None:
    assert compute_ath_and_lowest_since_ath([]) == (0.0, 0.0)


def test_compute_ath_and_lowest_since_ath_single() -> None:
    assert compute_ath_and_lowest_since_ath([100.0]) == (100.0, 100.0)


def test_compute_ath_and_lowest_since_ath_uptrend_then_drawdown() -> None:
    # 100 -> 120 (ATH) -> 110 (lowest since ATH) -> 115
    closes = [100.0, 120.0, 110.0, 115.0]
    ath, lowest = compute_ath_and_lowest_since_ath(closes)
    assert ath == 120.0
    assert lowest == 110.0


def test_compute_ath_and_lowest_since_ath_new_ath_resets_lowest() -> None:
    closes = [100.0, 90.0, 95.0, 105.0, 102.0]
    ath, lowest = compute_ath_and_lowest_since_ath(closes)
    assert ath == 105.0
    assert lowest == 102.0
