"""Parser for the Financial Instrument Information section.

Must be run FIRST — its alias map is passed to positions and trades parsers.
"""

from __future__ import annotations

from typing import Any


def parse_instruments(sections: dict[str, list[dict[str, Any]]]) -> dict[str, str]:
    """Return alias_map: {alias_symbol: canonical_symbol}.

    IBKR represents symbol aliases as 'CSSPXz, CSPX' in the Symbol column,
    where the first token is the alias and the second is the canonical name.
    """
    alias_map: dict[str, str] = {}
    for row in sections.get("Financial Instrument Information", []):
        symbol_field = row.get("Symbol", "").strip()
        if "," in symbol_field:
            parts = [p.strip() for p in symbol_field.split(",", 1)]
            alias, canonical = parts[0], parts[1]
            alias_map[alias] = canonical
    return alias_map


def build_instrument_info(
    sections: dict[str, list[dict[str, Any]]],
    alias_map: dict[str, str],
) -> dict[str, dict[str, str]]:
    """Return {canonical_symbol: {description, isin, instrument_type}}."""
    info: dict[str, dict[str, str]] = {}
    for row in sections.get("Financial Instrument Information", []):
        symbol_field = str(row.get("Symbol", "")).strip()
        if "," in symbol_field:
            canonical: str = symbol_field.split(",", 1)[1].strip()
        else:
            canonical = alias_map.get(symbol_field, symbol_field)
        info[canonical] = {
            "description": row.get("Description", ""),
            "isin": row.get("Security ID", ""),
            "instrument_type": row.get("Type", ""),
        }
    return info
