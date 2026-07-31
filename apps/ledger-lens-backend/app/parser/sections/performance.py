"""Parsers for Realized & Unrealized P&L, Mark-to-Market, and Corporate Actions."""

from __future__ import annotations

import re
from typing import Any

from app.parser.normalizers import normalize_symbol, parse_date, parse_datetime, parse_float_or_zero

_CORP_ACTION_RE = re.compile(r"^(?P<symbol>\w+)\((?P<isin>[A-Z0-9]+)\)\s+(?P<action>.+)")

# Rows whose Symbol column is empty or whose Asset Category marks them as aggregate totals
_SKIP_CATEGORIES = frozenset({"Total", "Total (All Assets)", "Other Fees"})


def parse_pnl(
    sections: dict[str, list[dict[str, Any]]],
    alias_map: dict[str, str],
) -> list[dict[str, Any]]:
    """Return per-symbol realized & unrealized P&L rows (stocks only, not totals)."""
    records: list[dict[str, Any]] = []

    for row in sections.get("Realized & Unrealized Performance Summary", []):
        asset_category = row.get("Asset Category", "")
        symbol = row.get("Symbol", "").strip()

        if asset_category in _SKIP_CATEGORIES or not symbol:
            continue

        symbol = normalize_symbol(symbol, alias_map)

        records.append(
            {
                "symbol": symbol,
                "asset_category": asset_category,
                "realized_st_profit": parse_float_or_zero(row.get("Realized S/T Profit", "0")),
                "realized_st_loss": parse_float_or_zero(row.get("Realized S/T Loss", "0")),
                "realized_lt_profit": parse_float_or_zero(row.get("Realized L/T Profit", "0")),
                "realized_lt_loss": parse_float_or_zero(row.get("Realized L/T Loss", "0")),
                "realized_total": parse_float_or_zero(row.get("Realized Total", "0")),
                "unrealized_st_profit": parse_float_or_zero(row.get("Unrealized S/T Profit", "0")),
                "unrealized_st_loss": parse_float_or_zero(row.get("Unrealized S/T Loss", "0")),
                "unrealized_lt_profit": parse_float_or_zero(row.get("Unrealized L/T Profit", "0")),
                "unrealized_lt_loss": parse_float_or_zero(row.get("Unrealized L/T Loss", "0")),
                "unrealized_total": parse_float_or_zero(row.get("Unrealized Total", "0")),
                "total": parse_float_or_zero(row.get("Total", "0")),
            }
        )

    return records


def parse_mtm(
    sections: dict[str, list[dict[str, Any]]],
    alias_map: dict[str, str],
) -> list[dict[str, Any]]:
    """Return per-symbol Mark-to-Market rows (excludes total/aggregate rows)."""
    records: list[dict[str, Any]] = []

    for row in sections.get("Mark-to-Market Performance Summary", []):
        asset_category = row.get("Asset Category", "")
        symbol = row.get("Symbol", "").strip()

        if asset_category in _SKIP_CATEGORIES or not symbol:
            continue

        symbol = normalize_symbol(symbol, alias_map)

        records.append(
            {
                "symbol": symbol,
                "asset_category": asset_category,
                "prior_qty": parse_float_or_zero(row.get("Prior Quantity", "0")),
                "current_qty": parse_float_or_zero(row.get("Current Quantity", "0")),
                "prior_price": parse_float_or_zero(row.get("Prior Price", "0")),
                "current_price": parse_float_or_zero(row.get("Current Price", "0")),
                "mtm_position": parse_float_or_zero(row.get("Mark-to-Market P/L Position", "0")),
                "mtm_transaction": parse_float_or_zero(
                    row.get("Mark-to-Market P/L Transaction", "0")
                ),
                "mtm_commissions": parse_float_or_zero(
                    row.get("Mark-to-Market P/L Commissions", "0")
                ),
                "mtm_other": parse_float_or_zero(row.get("Mark-to-Market P/L Other", "0")),
                "mtm_total": parse_float_or_zero(row.get("Mark-to-Market P/L Total", "0")),
            }
        )

    return records


def parse_corporate_actions(
    sections: dict[str, list[dict[str, Any]]],
) -> list[dict[str, Any]]:
    """Return corporate action events (splits, etc.), excluding total rows."""
    actions: list[dict[str, Any]] = []

    for row in sections.get("Corporate Actions", []):
        asset_category = row.get("Asset Category", "")
        if asset_category in _SKIP_CATEGORIES or not asset_category:
            continue

        report_date = parse_date(row.get("Report Date", ""))
        dt = parse_datetime(row.get("Date/Time", ""))
        action_date = dt.date() if dt else report_date
        if action_date is None:
            continue

        description = row.get("Description", "")
        symbol, isin = _parse_corp_action_desc(description)

        actions.append(
            {
                "action_date": action_date,
                "symbol": symbol,
                "isin": isin,
                "description": description,
                "quantity": parse_float_or_zero(row.get("Quantity", "0")),
                "proceeds": parse_float_or_zero(row.get("Proceeds", "0")),
                "value": parse_float_or_zero(row.get("Value", "0")),
                "realized_pnl": parse_float_or_zero(row.get("Realized P/L", "0")),
                "currency": row.get("Currency", ""),
            }
        )

    return actions


def _parse_corp_action_desc(description: str) -> tuple[str, str | None]:
    """Extract (symbol, isin) from e.g. 'IBKR(US45841N1072) Split 4 for 1 ...'."""
    m = _CORP_ACTION_RE.match(description)
    if m:
        return m.group("symbol"), m.group("isin")
    token = description.split("(")[0].strip()
    return token or description, None
