"""Parser for the Open Positions section."""

from __future__ import annotations

from typing import Any

from app.parser.normalizers import normalize_symbol, parse_float, parse_float_or_zero


def parse_positions(
    sections: dict[str, list[dict[str, Any]]],
    alias_map: dict[str, str],
) -> list[dict[str, Any]]:
    """Return list of open positions, alias-normalized, excluding total rows."""
    positions: list[dict[str, Any]] = []

    for row in sections.get("Open Positions", []):
        if row.get("DataDiscriminator") != "Summary":
            continue

        symbol = normalize_symbol(row.get("Symbol", ""), alias_map)
        close_price_raw = row.get("Close Price", "")

        positions.append(
            {
                "symbol": symbol,
                "asset_category": row.get("Asset Category", "Stocks"),
                "currency": row.get("Currency", "USD"),
                "quantity": parse_float_or_zero(row.get("Quantity", "0")),
                "cost_price": parse_float_or_zero(row.get("Cost Price", "0")),
                "cost_basis": parse_float_or_zero(row.get("Cost Basis", "0")),
                # '--' means the position was closed (no current price)
                "close_price": parse_float(close_price_raw) or 0.0,
                "current_value": parse_float_or_zero(row.get("Value", "0")),
                "unrealized_pnl": parse_float_or_zero(row.get("Unrealized P/L", "0")),
            }
        )

    return positions
