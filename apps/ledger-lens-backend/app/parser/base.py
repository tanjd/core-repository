"""IBKR activity statement CSV section-splitting reader.

The IBKR CSV format is NOT a flat CSV. It embeds ~15 named tables, each with:
    SectionName,Header,col1,col2,...   <- schema definition
    SectionName,Data,val1,val2,...     <- actual data rows
    SectionName,SubTotal,...           <- skipped
    SectionName,Total,...              <- skipped

Multiple Header rows within the same section (e.g. Trades has two: one for
stocks and one for forex) are handled by updating the column names when a new
Header row is encountered; all subsequent Data rows use those new columns.
"""

from __future__ import annotations

import csv
from pathlib import Path
from typing import Any

# {section_name: [row_dict, ...]}  — only Data rows are included
ParsedCSV = dict[str, list[dict[str, Any]]]


def parse_ibkr_csv(filepath: str | Path) -> ParsedCSV:
    """Parse an IBKR activity statement CSV into a dict of section → [row_dict].

    Handles:
    - UTF-8 BOM (utf-8-sig)
    - Multiple Header rows per section (updates column names in place)
    - Only includes Data rows; SubTotal / Total / Header rows are dropped
    """
    sections: ParsedCSV = {}
    current_headers: dict[str, list[str]] = {}

    with open(filepath, newline="", encoding="utf-8-sig") as f:
        reader = csv.reader(f)
        for row in reader:
            if len(row) < 2:
                continue

            section = row[0].strip()
            row_type = row[1].strip()
            fields = row[2:]

            if row_type == "Header":
                current_headers[section] = fields
            elif row_type == "Data":
                headers = current_headers.get(section, [])
                # Ensure fields list is at least as long as headers
                while len(fields) < len(headers):
                    fields.append("")
                row_dict = dict(zip(headers, fields, strict=False))
                sections.setdefault(section, []).append(row_dict)
            # SubTotal / Total / Notes rows are intentionally skipped

    return sections
