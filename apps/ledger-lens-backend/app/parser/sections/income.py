"""Parsers for Dividends, Withholding Tax, Fees, and Commission Adjustments."""

from __future__ import annotations

import re
from typing import Any

from app.parser.normalizers import parse_date, parse_float_or_zero

_DIVIDEND_RE = re.compile(
    r"^(?P<symbol>\w+)\((?P<isin>[A-Z0-9]+)\)"
    r" Cash Dividend \w+ (?P<rate>[\d.]+) per Share"
    r" \((?P<dtype>[^)]+)\)"
)

_WITHHOLDING_RE = re.compile(
    r"^(?P<symbol>\w+)\((?P<isin>[A-Z0-9]+)\)" r" Cash Dividend \w+ (?P<rate>[\d.]+) per Share"
)


def parse_dividends(sections: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
    """Return individual dividend payments, excluding Total rows."""
    dividends: list[dict[str, Any]] = []

    for row in sections.get("Dividends", []):
        currency = row.get("Currency", "")
        if currency in ("Total", ""):
            continue

        pay_date = parse_date(row.get("Date", ""))
        if pay_date is None:
            continue

        description = row.get("Description", "")
        symbol, isin, per_share_rate, dividend_type = _parse_dividend_desc(description)

        dividends.append(
            {
                "pay_date": pay_date,
                "currency": currency,
                "symbol": symbol,
                "isin": isin,
                "per_share_rate": per_share_rate,
                "gross_amount": parse_float_or_zero(row.get("Amount", "0")),
                "dividend_type": dividend_type,
                "description": description,
            }
        )

    return dividends


def parse_withholding_tax(sections: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
    """Return withholding tax entries (including carry-overs from prior year)."""
    taxes: list[dict[str, Any]] = []

    for row in sections.get("Withholding Tax", []):
        currency = row.get("Currency", "")
        if currency in ("Total", ""):
            continue

        tax_date = parse_date(row.get("Date", ""))
        if tax_date is None:
            continue

        description = row.get("Description", "")
        symbol, isin, _, _ = _parse_dividend_desc(description)

        taxes.append(
            {
                "tax_date": tax_date,
                "currency": currency,
                "symbol": symbol,
                "isin": isin,
                "amount": parse_float_or_zero(row.get("Amount", "0")),
                "description": description,
            }
        )

    return taxes


def parse_fees(sections: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
    """Return entries from Fees and Commission Adjustments sections."""
    fees: list[dict[str, Any]] = []

    for row in sections.get("Fees", []):
        currency = row.get("Currency", "")
        if currency in ("Total", ""):
            continue

        fee_date = parse_date(row.get("Date", ""))
        if fee_date is None:
            continue

        fees.append(
            {
                "fee_date": fee_date,
                "currency": currency,
                "description": row.get("Description", ""),
                "amount": parse_float_or_zero(row.get("Amount", "0")),
                "fee_type": row.get("Subtitle", "Other Fees"),
            }
        )

    for row in sections.get("Commission Adjustments", []):
        currency = row.get("Currency", "")
        if currency in ("Total", ""):
            continue

        fee_date = parse_date(row.get("Date", ""))
        if fee_date is None:
            continue

        fees.append(
            {
                "fee_date": fee_date,
                "currency": currency,
                "description": row.get("Description", ""),
                "amount": parse_float_or_zero(row.get("Amount", "0")),
                "fee_type": "Commission Adjustment",
            }
        )

    return fees


def _parse_dividend_desc(
    description: str,
) -> tuple[str, str | None, float, str]:
    """Extract (symbol, isin, per_share_rate, dividend_type) from description text."""
    m = _DIVIDEND_RE.match(description)
    if m:
        return (
            m.group("symbol"),
            m.group("isin"),
            float(m.group("rate")),
            m.group("dtype"),
        )
    # Withholding tax descriptions don't have the type suffix
    m2 = _WITHHOLDING_RE.match(description)
    if m2:
        return (
            m2.group("symbol"),
            m2.group("isin"),
            float(m2.group("rate")),
            "Withholding Tax",
        )
    # Fallback: try to extract leading symbol token
    token = description.split("(")[0].strip()
    return (token or description, None, 0.0, "Unknown")
