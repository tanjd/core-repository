"""Parser for the Deposits & Withdrawals section."""

from __future__ import annotations

from typing import Any

from app.parser.normalizers import parse_date, parse_float_or_zero

# These are summary/subtotal rows embedded as Data rows in this section
_SKIP_CURRENCIES = frozenset({"Total", "Total in USD", "Total Deposits & Withdrawals in USD"})


def parse_deposits(sections: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
    """Return individual deposit/withdrawal rows, excluding subtotal rows."""
    deposits: list[dict[str, Any]] = []

    for row in sections.get("Deposits & Withdrawals", []):
        currency = row.get("Currency", "").strip()
        if currency in _SKIP_CURRENCIES:
            continue

        settle_date = parse_date(row.get("Settle Date", ""))
        if settle_date is None:
            continue

        deposits.append(
            {
                "settle_date": settle_date,
                "currency": currency,
                "description": row.get("Description", ""),
                "amount": parse_float_or_zero(row.get("Amount", "0")),
            }
        )

    return deposits
