"""Parser for the Trades section.

The Trades section contains TWO Header rows:
  1. Stock trades: standard 14-column layout
  2. Forex trades: different column names (empty placeholders where stock columns don't apply)

Both sets of Data rows land in the same list from base.py, distinguished by
'Asset Category' == 'Stocks' vs 'Forex'.
"""

from __future__ import annotations

import json
from typing import Any

from app.parser.normalizers import normalize_symbol, parse_datetime, parse_float_or_zero


def parse_trades(
    sections: dict[str, list[dict[str, Any]]],
    alias_map: dict[str, str],
) -> list[dict[str, Any]]:
    """Return all trades (stocks + forex), alias-normalized.

    Only 'Order' DataDiscriminator rows are included; SubTotal rows are already
    excluded by the base parser (they have row_type='SubTotal', not 'Data').
    """
    trades: list[dict[str, Any]] = []

    for row in sections.get("Trades", []):
        if row.get("DataDiscriminator") != "Order":
            continue

        asset_category = row.get("Asset Category", "")

        if asset_category == "Stocks":
            trade = _parse_stock_trade(row, alias_map)
        elif asset_category == "Forex":
            trade = _parse_forex_trade(row)
        else:
            continue

        if trade is not None:
            trades.append(trade)

    return trades


def _parse_stock_trade(
    row: dict[str, Any],
    alias_map: dict[str, str],
) -> dict[str, Any] | None:
    symbol = normalize_symbol(row.get("Symbol", ""), alias_map)
    dt = parse_datetime(row.get("Date/Time", ""))
    if dt is None:
        return None

    quantity = parse_float_or_zero(row.get("Quantity", "0"))
    codes = _parse_codes(row.get("Code", ""))

    return {
        "asset_category": "Stocks",
        "currency": row.get("Currency", "USD"),
        "symbol": symbol,
        "trade_date": dt,
        "quantity": quantity,
        "trade_price": parse_float_or_zero(row.get("T. Price", "0")),
        "close_price": parse_float_or_zero(row.get("C. Price", "0")),
        "proceeds": parse_float_or_zero(row.get("Proceeds", "0")),
        "commission": parse_float_or_zero(row.get("Comm/Fee", "0")),
        "basis": parse_float_or_zero(row.get("Basis", "0")),
        "realized_pnl": parse_float_or_zero(row.get("Realized P/L", "0")),
        "mtm_pnl": parse_float_or_zero(row.get("MTM P/L", "0")),
        "codes": json.dumps(codes),
        "direction": "buy" if quantity >= 0 else "sell",
    }


def _parse_forex_trade(row: dict[str, Any]) -> dict[str, Any] | None:
    symbol = row.get("Symbol", "").strip()
    dt = parse_datetime(row.get("Date/Time", ""))
    if dt is None:
        return None

    quantity = parse_float_or_zero(row.get("Quantity", "0"))
    codes = _parse_codes(row.get("Code", ""))

    return {
        "asset_category": "Forex",
        "currency": row.get("Currency", ""),
        "symbol": symbol,
        "trade_date": dt,
        "quantity": quantity,
        "trade_price": parse_float_or_zero(row.get("T. Price", "0")),
        "close_price": 0.0,
        "proceeds": parse_float_or_zero(row.get("Proceeds", "0")),
        "commission": parse_float_or_zero(row.get("Comm in USD", "0")),
        "basis": 0.0,
        "realized_pnl": 0.0,
        "mtm_pnl": parse_float_or_zero(row.get("MTM in USD", "0")),
        "codes": json.dumps(codes),
        "direction": "buy" if quantity >= 0 else "sell",
    }


def _parse_codes(raw: str) -> list[str]:
    """Split 'O;RI;FPA' → ['O', 'RI', 'FPA'], ignoring empty strings."""
    return [c.strip() for c in raw.split(";") if c.strip()]
