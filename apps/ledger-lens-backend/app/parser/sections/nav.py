"""Parser for Net Asset Value and Change in NAV sections."""

from __future__ import annotations

from typing import Any

from app.parser.normalizers import parse_float, parse_float_or_zero


def parse_nav(sections: dict[str, list[dict[str, Any]]]) -> dict[str, Any]:
    """Extract NAV totals, per-asset-class breakdown, TWR, and Change in NAV fields."""
    nav_rows = sections.get("Net Asset Value", [])
    change_rows_raw = sections.get("Change in NAV", [])
    change_rows = {r["Field Name"]: r["Field Value"] for r in change_rows_raw}

    nav_prior = 0.0
    nav_current = 0.0
    nav_change = 0.0
    nav_stock = 0.0
    nav_cash = 0.0
    twr_pct = 0.0

    for row in nav_rows:
        if "Time Weighted Rate of Return" in row:
            twr_pct = parse_float(row["Time Weighted Rate of Return"]) or 0.0
        elif "Asset Class" in row:
            asset_class = row["Asset Class"].strip()
            if asset_class == "Total":
                nav_prior = parse_float_or_zero(row.get("Prior Total", "0"))
                nav_current = parse_float_or_zero(row.get("Current Total", "0"))
                nav_change = parse_float_or_zero(row.get("Change", "0"))
            elif asset_class in ("Stock", "Stocks"):
                nav_stock = parse_float_or_zero(row.get("Current Total", "0"))
            elif asset_class in ("Cash", "Cash "):
                nav_cash = parse_float_or_zero(row.get("Current Total", "0"))

    return {
        "twr_pct": twr_pct,
        "nav_prior": nav_prior,
        "nav_current": nav_current,
        "nav_change": nav_change,
        "nav_stock": nav_stock,
        "nav_cash": nav_cash,
        "starting_value": parse_float_or_zero(change_rows.get("Starting Value", "0")),
        "ending_value": parse_float_or_zero(change_rows.get("Ending Value", "0")),
        "deposits_withdrawals": parse_float_or_zero(change_rows.get("Deposits & Withdrawals", "0")),
        "mark_to_market": parse_float_or_zero(change_rows.get("Mark-to-Market", "0")),
        "dividends": parse_float_or_zero(change_rows.get("Dividends", "0")),
        "withholding_tax": parse_float_or_zero(change_rows.get("Withholding Tax", "0")),
        "commissions": parse_float_or_zero(change_rows.get("Commissions", "0")),
        "other_fees": parse_float_or_zero(change_rows.get("Other Fees", "0")),
        "sales_tax": parse_float_or_zero(change_rows.get("Sales Tax", "0")),
    }
