"""Parser for Statement and Account Information sections."""

from __future__ import annotations

import re
from datetime import date, datetime
from typing import Any


def parse_statement_meta(sections: dict[str, list[dict[str, Any]]]) -> dict[str, Any]:
    """Extract account metadata and statement period from the CSV."""
    stmt_rows = {r["Field Name"]: r["Field Value"] for r in sections.get("Statement", [])}
    acct_rows = {r["Field Name"]: r["Field Value"] for r in sections.get("Account Information", [])}

    period = stmt_rows.get("Period", "")
    year = _extract_year(period)
    period_start, period_end = _parse_period_dates(period)

    return {
        "period": period,
        "year": year,
        "period_start": period_start,
        "period_end": period_end,
        "account_id": acct_rows.get("Account", ""),
        "account_name": acct_rows.get("Name", ""),
        "base_currency": acct_rows.get("Base Currency", "USD"),
    }


def _extract_year(period: str) -> int:
    """Extract the ending year from 'January 1, 2025 - December 31, 2025'."""
    # Take the last 4-digit year found in the string
    matches = re.findall(r"\b(\d{4})\b", period)
    if matches:
        return int(matches[-1])
    raise ValueError(f"Cannot extract year from period: {period!r}")


def _parse_period_dates(period: str) -> tuple[date | None, date | None]:
    """Parse 'January 1, 2026 - March 2, 2026' into (date(2026,1,1), date(2026,3,2)).

    Returns (None, None) if the period string cannot be parsed.
    """
    if " - " not in period:
        return None, None
    parts = period.split(" - ", 1)
    try:
        start = datetime.strptime(parts[0].strip(), "%B %d, %Y").date()
        end = datetime.strptime(parts[1].strip(), "%B %d, %Y").date()
        return start, end
    except ValueError:
        return None, None
