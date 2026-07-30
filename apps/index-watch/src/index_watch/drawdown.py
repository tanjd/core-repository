"""Drawdown metrics calculation from price series."""

from dataclasses import dataclass


@dataclass
class DrawdownMetrics:
    """Drawdown metrics for an index."""

    current_price: float
    ath: float
    current_drawdown_pct: float
    lowest_since_ath: float
    drawdown_at_lowest_pct: float
    gain_from_lowest_pct: float
    gain_to_ath_from_current_pct: float
    gain_to_ath_from_lowest_pct: float


def compute_drawdown_metrics(
    current_price: float,
    ath: float,
    lowest_since_ath: float,
) -> DrawdownMetrics:
    """Compute drawdown metrics from current price, ATH, and lowest since ATH."""
    if ath <= 0:
        raise ValueError("ATH must be positive")

    current_drawdown_pct = (current_price / ath - 1) * 100 if ath else 0.0
    drawdown_at_lowest_pct = (lowest_since_ath / ath - 1) * 100 if ath else 0.0

    gain_from_lowest_pct = (
        (current_price / lowest_since_ath - 1) * 100
        if lowest_since_ath and lowest_since_ath > 0
        else 0.0
    )
    gain_to_ath_from_current_pct = (
        (ath / current_price - 1) * 100 if current_price and current_price > 0 else 0.0
    )
    gain_to_ath_from_lowest_pct = (
        (ath / lowest_since_ath - 1) * 100 if lowest_since_ath and lowest_since_ath > 0 else 0.0
    )

    return DrawdownMetrics(
        current_price=current_price,
        ath=ath,
        current_drawdown_pct=current_drawdown_pct,
        lowest_since_ath=lowest_since_ath,
        drawdown_at_lowest_pct=drawdown_at_lowest_pct,
        gain_from_lowest_pct=gain_from_lowest_pct,
        gain_to_ath_from_current_pct=gain_to_ath_from_current_pct,
        gain_to_ath_from_lowest_pct=gain_to_ath_from_lowest_pct,
    )


def compute_ath_and_lowest_since_ath(closes: list[float]) -> tuple[float, float]:
    """Chronological closes (oldest first) -> (ATH, lowest since ATH)."""
    if not closes:
        return 0.0, 0.0
    ath = closes[0]
    lowest_since_ath = closes[0]
    for p in closes[1:]:
        if p > ath:
            ath = p
            lowest_since_ath = p
        elif p < lowest_since_ath:
            lowest_since_ath = p
    return ath, lowest_since_ath
