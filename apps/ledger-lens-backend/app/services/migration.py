"""One-time data directory migration helpers.

Moves legacy flat IBKR CSVs from data/ root into data/ibkr/ subdirectory.
Runs at startup; safe to run on every restart (idempotent).
"""

from __future__ import annotations

import logging
import re
from pathlib import Path

logger = logging.getLogger(__name__)

_IBKR_RE = re.compile(r"^[A-Z]\d+_\d{4}(?:\d{4})?_\d{4}(?:\d{4})?\.csv$")


def migrate_data_dir(data_dir: str) -> None:
    """Move legacy flat IBKR CSVs from data/ root into data/ibkr/."""
    root = Path(data_dir)
    ibkr_dir = root / "ibkr"
    ibkr_dir.mkdir(parents=True, exist_ok=True)

    moved = 0
    for csv in root.glob("*.csv"):
        if _IBKR_RE.match(csv.name):
            dest = ibkr_dir / csv.name
            csv.rename(dest)
            logger.info("Migrated %s → ibkr/", csv.name)
            moved += 1

    if moved:
        logger.info("Migration complete: moved %d IBKR file(s) to ibkr/", moved)
