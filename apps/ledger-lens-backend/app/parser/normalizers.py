"""Shared parsing helpers for IBKR CSV field values."""

from __future__ import annotations

from datetime import date, datetime


def parse_float(value: str) -> float | None:
    """Convert an IBKR number string to float.

    Handles:
    - Empty string / whitespace → None
    - '--' sentinel (N/A) → None
    - Quoted comma-numbers: '3,154.99' → 3154.99 (csv.reader unquotes; we strip commas)
    - Percentage strings: '17.849587745%' → 17.849587745
    """
    cleaned = value.strip().rstrip("%")
    if not cleaned or cleaned == "--":
        return None
    try:
        return float(cleaned.replace(",", ""))
    except ValueError:
        return None


def parse_float_req(value: str, field: str = "") -> float:
    """parse_float that raises ValueError when the value is missing or invalid."""
    result = parse_float(value)
    if result is None:
        label = f" ({field})" if field else ""
        raise ValueError(f"Expected a number{label}, got: {value!r}")
    return result


def parse_float_or_zero(value: str) -> float:
    """parse_float that returns 0.0 instead of None."""
    return parse_float(value) or 0.0


def parse_date(value: str) -> date | None:
    """Parse IBKR date strings.

    Accepts:
    - '2025-01-02'
    - '2025-01-02, 11:22:01'  (truncates time)
    """
    cleaned = value.strip()
    if not cleaned or cleaned == "--":
        return None
    if ", " in cleaned:
        try:
            return datetime.strptime(cleaned, "%Y-%m-%d, %H:%M:%S").date()
        except ValueError:
            pass
    try:
        return date.fromisoformat(cleaned)
    except ValueError:
        return None


def parse_datetime(value: str) -> datetime | None:
    """Parse IBKR datetime string '2025-01-02, 11:22:01'."""
    cleaned = value.strip()
    if not cleaned or cleaned == "--":
        return None
    try:
        return datetime.strptime(cleaned, "%Y-%m-%d, %H:%M:%S")
    except ValueError:
        return None


def normalize_symbol(symbol: str, alias_map: dict[str, str]) -> str:
    """Map alias symbols to their canonical names (e.g. CSSPXz → CSPX)."""
    return alias_map.get(symbol.strip(), symbol.strip())
