"""Unit tests for the dependency-free XIRR solver (app.services.returns.xirr)."""

from __future__ import annotations

from datetime import date

import pytest
from app.services.returns import xirr


def test_single_deposit_one_year_ten_percent():
    cash_flows = [(date(2024, 1, 1), -1000.0), (date(2025, 1, 1), 1100.0)]
    rate = xirr(cash_flows)
    assert rate is not None
    assert rate == pytest.approx(10.0, abs=0.05)


def test_multiple_deposits_and_terminal_value():
    # Deposit 1000 on day 0, another 1000 six months later, worth 2200 one year after the first.
    cash_flows = [
        (date(2024, 1, 1), -1000.0),
        (date(2024, 7, 1), -1000.0),
        (date(2025, 1, 1), 2200.0),
    ]
    rate = xirr(cash_flows)
    assert rate is not None
    # Sanity: a two-deposit portfolio worth 2200 on ~2000 invested is a healthy double-digit
    # annualized return, not a wildly wrong number.
    assert 5.0 < rate < 40.0


def test_no_sign_change_returns_none():
    cash_flows = [(date(2024, 1, 1), 1000.0), (date(2025, 1, 1), 1100.0)]
    assert xirr(cash_flows) is None


def test_single_date_returns_none():
    cash_flows = [(date(2024, 1, 1), -1000.0), (date(2024, 1, 1), 1000.0)]
    assert xirr(cash_flows) is None


def test_empty_returns_none():
    assert xirr([]) is None
