"""Tests for message formatting."""

from index_watch.drawdown import DrawdownMetrics
from index_watch.fear_greed import FearGreedResult
from index_watch.formatting import (
    format_drawdown_alert,
    format_drawdown_block,
    format_fear_greed,
    format_historical_frequency,
)


def test_format_drawdown_block() -> None:
    m = DrawdownMetrics(
        current_price=6836.17,
        ath=7002.28,
        current_drawdown_pct=-2.37,
        lowest_since_ath=6780.13,
        drawdown_at_lowest_pct=-3.17,
        gain_from_lowest_pct=0.83,
        gain_to_ath_from_current_pct=2.43,
        gain_to_ath_from_lowest_pct=3.28,
    )
    text = format_drawdown_block("S&P 500", m)
    assert "📊 S&P 500" in text
    assert "-2.37" in text
    assert "6,836.17" in text
    assert "7,002.28" in text
    assert "🟢" in text  # Green emoji for healthy drawdown


def test_format_fear_greed_none() -> None:
    assert "unavailable" in format_fear_greed(None)


def test_format_fear_greed_value() -> None:
    fg = FearGreedResult(value=25.0, description="Fear", last_update="2024-01-15")
    text = format_fear_greed(fg)
    assert "25.0" in text
    assert "Fear" in text


def test_format_historical_frequency() -> None:
    text = format_historical_frequency("S&P 500", (5, 10), {5: 100, 10: 30}, 1000)
    assert "S&P 500" in text
    assert "5%" in text
    assert "100 days" in text
    assert "10.0%" in text
    assert "🟢" in text  # Green emoji for 5% threshold


def test_format_drawdown_alert() -> None:
    text = format_drawdown_alert("S&P 500", -7.5, 5, 120, 5000)
    assert "S&P 500" in text
    assert "-7.50" in text
    assert "5%" in text
    assert "120" in text
