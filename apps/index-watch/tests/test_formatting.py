"""Tests for message formatting."""

from index_watch.alerts import AlertDetail, RecoveryDetail
from index_watch.drawdown import DrawdownMetrics
from index_watch.fear_greed import FearGreedResult
from index_watch.formatting import (
    format_alert_digest,
    format_daily_report,
    format_drawdown_block,
    format_drawdown_digest,
    format_fear_greed,
    format_historical_frequency,
    format_recovery_digest,
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


def test_format_drawdown_digest_single_alert() -> None:
    alerts = [
        AlertDetail(
            symbol="^GSPC",
            name="S&P 500",
            current_drawdown_pct=-7.5,
            threshold_pct=5,
            day_count=120,
            total_days=5000,
            history_years=30,
        )
    ]
    text = format_drawdown_digest(alerts)
    assert "Drawdown Alert (1)" in text
    assert "S&P 500" in text
    assert "-7.50" in text
    assert "5%" in text
    assert "120" in text
    assert "In the last 30 years" in text


def test_format_drawdown_digest_multiple_alerts() -> None:
    alerts = [
        AlertDetail(
            symbol="^GSPC",
            name="S&P 500",
            current_drawdown_pct=-7.5,
            threshold_pct=5,
            day_count=120,
            total_days=5000,
            history_years=30,
        ),
        AlertDetail(
            symbol="^NDX",
            name="NASDAQ-100",
            current_drawdown_pct=-12.0,
            threshold_pct=10,
            day_count=80,
            total_days=5000,
            history_years=30,
        ),
    ]
    text = format_drawdown_digest(alerts)
    assert "Drawdown Alerts (2)" in text
    assert "S&P 500" in text
    assert "NASDAQ-100" in text
    # Both sections present, separated by a divider
    assert text.count("Historical Context") == 2


def test_format_recovery_digest_new_ath() -> None:
    recoveries = [
        RecoveryDetail(
            symbol="^GSPC", name="S&P 500", current_price=7100.0, ath=7100.0, is_new_ath=True
        )
    ]
    text = format_recovery_digest(recoveries)
    assert "Recovery / New Highs" in text
    assert "new all-time high" in text
    assert "S&P 500" in text
    assert "7,100.00" in text


def test_format_recovery_digest_recovered_to_previous_ath() -> None:
    recoveries = [
        RecoveryDetail(
            symbol="^GSPC", name="S&P 500", current_price=7002.28, ath=7002.28, is_new_ath=False
        )
    ]
    text = format_recovery_digest(recoveries)
    assert "recovered to its previous all-time high" in text


def test_format_alert_digest_alerts_only() -> None:
    alerts = [
        AlertDetail(
            symbol="^GSPC",
            name="S&P 500",
            current_drawdown_pct=-7.5,
            threshold_pct=5,
            day_count=120,
            total_days=5000,
            history_years=30,
        )
    ]
    text = format_alert_digest(alerts, [])
    assert "Drawdown Alert (1)" in text
    assert "Recovery / New Highs" not in text


def test_format_alert_digest_recoveries_only() -> None:
    recoveries = [
        RecoveryDetail(
            symbol="^GSPC", name="S&P 500", current_price=7100.0, ath=7100.0, is_new_ath=True
        )
    ]
    text = format_alert_digest([], recoveries)
    assert "Drawdown Alert" not in text
    assert "Recovery / New Highs" in text


def test_format_alert_digest_both_sections_present() -> None:
    alerts = [
        AlertDetail(
            symbol="^GSPC",
            name="S&P 500",
            current_drawdown_pct=-7.5,
            threshold_pct=5,
            day_count=120,
            total_days=5000,
            history_years=30,
        )
    ]
    recoveries = [
        RecoveryDetail(
            symbol="^NDX", name="NASDAQ-100", current_price=20000.0, ath=20000.0, is_new_ath=True
        )
    ]
    text = format_alert_digest(alerts, recoveries)
    assert "Drawdown Alert (1)" in text
    assert "Recovery / New Highs" in text
    assert "S&P 500" in text
    assert "NASDAQ-100" in text


def test_format_daily_report_history_years() -> None:
    text = format_daily_report(
        [("S&P 500", "block")],
        "fear greed line",
        ["history block"],
        history_years=15,
    )
    assert "(Last 15 years)" in text
