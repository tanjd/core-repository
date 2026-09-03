"""Money-weighted return (XIRR) — a small dependency-free Newton-Raphson solver.

No numerical library (numpy/scipy) is in this app's dependencies; XIRR only needs a
one-dimensional root find, which a short Newton-Raphson-with-bisection-fallback covers without
adding one.
"""

from __future__ import annotations

from datetime import date

_DAYS_PER_YEAR = 365.0
_MAX_NEWTON_ITER = 100
_MAX_BISECT_ITER = 200
_TOLERANCE = 1e-6
# Bounds for the bisection fallback: -99.99% to +10,000% annualized.
_RATE_LO = -0.9999
_RATE_HI = 100.0


def _years_between(d0: date, d1: date) -> float:
    return (d1 - d0).days / _DAYS_PER_YEAR


def _npv(rate: float, cash_flows: list[tuple[date, float]], d0: date) -> float:
    return sum(amount / (1.0 + rate) ** _years_between(d0, dt) for dt, amount in cash_flows)


def _npv_derivative(rate: float, cash_flows: list[tuple[date, float]], d0: date) -> float:
    return sum(
        -_years_between(d0, dt) * amount / (1.0 + rate) ** (_years_between(d0, dt) + 1)
        for dt, amount in cash_flows
        if dt != d0
    )


def xirr(cash_flows: list[tuple[date, float]]) -> float | None:
    """Solve for the annualized rate that zeroes the NPV of `cash_flows`.

    `cash_flows` is a list of (date, signed_amount) pairs — negative for money leaving the
    investor's pocket, positive for money returned to them (see the callers for the exact sign
    convention used). Returns the rate as a percentage (e.g. 12.3 for 12.3%/year), or None when
    there isn't enough information to solve (fewer than two distinct dates, or no sign change —
    an all-positive or all-negative cash flow series has no finite root).
    """
    distinct_dates = {dt for dt, _ in cash_flows}
    if len(distinct_dates) < 2:
        return None
    if not any(amount > 0 for _, amount in cash_flows) or not any(
        amount < 0 for _, amount in cash_flows
    ):
        return None

    d0 = min(dt for dt, _ in cash_flows)

    # Newton-Raphson from a flat 10%/year guess.
    rate = 0.1
    for _ in range(_MAX_NEWTON_ITER):
        npv = _npv(rate, cash_flows, d0)
        if abs(npv) < _TOLERANCE:
            return rate * 100
        deriv = _npv_derivative(rate, cash_flows, d0)
        if deriv == 0:
            break
        next_rate = rate - npv / deriv
        # Newton stepped outside the domain where (1+rate) is even defined — bail to bisection.
        if next_rate <= _RATE_LO:
            break
        rate = next_rate

    # Bisection fallback — robust as long as the NPV function changes sign across the bracket.
    lo, hi = _RATE_LO, _RATE_HI
    npv_lo = _npv(lo, cash_flows, d0)
    npv_hi = _npv(hi, cash_flows, d0)
    if npv_lo == 0:
        return lo * 100
    if npv_hi == 0:
        return hi * 100
    if (npv_lo > 0) == (npv_hi > 0):
        # No sign change across the bracket — solver can't isolate a root.
        return None

    mid = (lo + hi) / 2
    for _ in range(_MAX_BISECT_ITER):
        mid = (lo + hi) / 2
        npv_mid = _npv(mid, cash_flows, d0)
        if abs(npv_mid) < _TOLERANCE:
            return mid * 100
        if (npv_mid > 0) == (npv_lo > 0):
            lo, npv_lo = mid, npv_mid
        else:
            hi = mid

    return mid * 100
