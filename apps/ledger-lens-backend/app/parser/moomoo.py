"""Moomoo (Futu SG) CSV export parsers.

Two file types are supported:
- Trade history:  History-Margin Account(XXXX)-YYYYMMDD-HHMMSS.csv
- Positions:      Positions-Margin Account(XXXX)-YYYYMMDD-HHMMSS.csv

Both are flat CSVs with a single header row (unlike the IBKR nested-section format).
"""

from __future__ import annotations

import csv
import re
from datetime import date, datetime
from pathlib import Path

from app.parser.normalizers import parse_float, parse_float_or_zero

_FILL_TIME_FMT = "%b %d, %Y %H:%M:%S ET"

_ACCOUNT_RE = re.compile(r"\((\d+)\)")
_DATE_RE = re.compile(r"-(\d{8})-")


def extract_moomoo_account_id(filepath: Path) -> str:
    """Extract account ID from moomoo filename, e.g. '0536'."""
    m = _ACCOUNT_RE.search(filepath.name)
    if not m:
        raise ValueError(f"Cannot extract account ID from filename: {filepath.name}")
    return m.group(1)


def extract_moomoo_year(filepath: Path) -> int:
    """Extract year from the 8-digit date component in the filename, e.g. 20260307 → 2026."""
    m = _DATE_RE.search(filepath.name)
    if not m:
        raise ValueError(f"Cannot extract year from filename: {filepath.name}")
    return int(m.group(1)[:4])


def extract_moomoo_date(filepath: Path) -> date | None:
    """Extract the export date from the filename, e.g. 20260307 → date(2026, 3, 7)."""
    m = _DATE_RE.search(filepath.name)
    if not m:
        return None
    raw = m.group(1)  # "20260307"
    try:
        return date(int(raw[:4]), int(raw[4:6]), int(raw[6:8]))
    except ValueError:
        return None


def _parse_fill_time(value: str) -> datetime:
    """Parse moomoo fill time string, e.g. 'Feb 11, 2026 09:31:05 ET'."""
    return datetime.strptime(value.strip(), _FILL_TIME_FMT)


def _parse_filled_at(value: str) -> tuple[float, float]:
    """Parse 'Filled@Avg Price' field, e.g. '1.1593@407.62' → (1.1593, 407.62)."""
    parts = value.strip().split("@")
    if len(parts) != 2:
        raise ValueError(f"Unexpected Filled@Avg Price format: {value!r}")
    return parse_float_or_zero(parts[0]), parse_float_or_zero(parts[1])


def parse_moomoo_trades(filepath: str | Path) -> dict[int, list[dict]]:
    """Parse a moomoo trade history CSV.

    Returns a dict keyed by calendar year, each value a list of trade dicts
    ready for ingestion. Only 'Filled' orders are included; cancelled/failed
    orders and blank-Side fill sub-rows are skipped.
    """
    result: dict[int, list[dict]] = {}

    with open(filepath, encoding="utf-8-sig", newline="") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            side = row.get("Side", "").strip()
            if not side:
                # Blank Side = fill detail sub-row — skip
                continue
            if row.get("Status", "").strip() != "Filled":
                continue

            fill_qty, fill_price = _parse_filled_at(row["Filled@Avg Price"])
            if fill_qty == 0.0:
                continue

            fill_time = _parse_fill_time(row["Fill Time"])
            year = fill_time.year

            fill_amount = parse_float_or_zero(row.get("Fill Amount", "0"))
            commission = -(parse_float_or_zero(row.get("Total", "0")))
            # Proceeds: positive for sells, negative for buys (convention matches IBKR)
            proceeds = fill_amount if side.lower() == "sell" else -fill_amount

            trade: dict = {
                "direction": "buy" if side.lower() == "buy" else "sell",
                "symbol": row["Symbol"].strip(),
                "trade_date": fill_time,
                "quantity": fill_qty,
                "trade_price": fill_price,
                "proceeds": proceeds,
                "commission": commission,
                "asset_category": "Stocks",
                "currency": row.get("Currency", "USD").strip() or "USD",
                "codes": "[]",
                "basis": 0.0,
                "realized_pnl": 0.0,
                "mtm_pnl": 0.0,
                "close_price": 0.0,
            }

            result.setdefault(year, []).append(trade)

    return result


def parse_moomoo_positions(filepath: str | Path) -> list[dict]:
    """Parse a moomoo positions snapshot CSV.

    Returns a list of position dicts ready for ingestion.
    Options-specific columns (greeks, IV) are ignored.
    """
    positions: list[dict] = []

    with open(filepath, encoding="utf-8-sig", newline="") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            symbol = row.get("Symbol", "").strip()
            if not symbol:
                continue

            diluted_cost = parse_float_or_zero(row.get("Diluted Cost", "0"))
            quantity = parse_float_or_zero(row.get("Quantity", "0"))
            pl_raw = row.get("P/L", "0").strip().lstrip("+")
            unrealized_pnl = parse_float(pl_raw) or 0.0

            positions.append(
                {
                    "symbol": symbol,
                    "description": row.get("Name", "").strip(),
                    "quantity": quantity,
                    "cost_price": diluted_cost,
                    "cost_basis": diluted_cost * quantity,
                    "close_price": parse_float_or_zero(row.get("Current price", "0")),
                    "current_value": parse_float_or_zero(row.get("Market Value", "0")),
                    "unrealized_pnl": unrealized_pnl,
                    "asset_category": "Stocks",
                    "currency": row.get("Currency", "USD").strip() or "USD",
                    "isin": None,
                }
            )

    return positions
