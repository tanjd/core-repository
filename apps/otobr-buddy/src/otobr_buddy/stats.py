"""Stats helpers: streaks and coverage summaries.

Simplifying assumption (see CLAUDE.md "Open questions"): a streak is measured in
fixed-length cycles of the partnership's configured frequency (the interval in days
for interval mode, or 7 days for weekly mode). A cycle counts as "met" if at least
one session was logged within it; a cycle with no session breaks the streak. The
current, still-in-progress cycle doesn't count against the streak until it ends.
"""

from dataclasses import dataclass
from datetime import datetime
from typing import Any

from otobr_buddy.timeutil import parse_timestamp, utcnow


@dataclass
class Streaks:
    current: int
    longest: int


def cycle_length_days(partnership: dict[str, Any]) -> int | None:
    """Resolve a partnership's reminder frequency into a fixed cycle length, in days.

    Returns None if no frequency has been configured yet (streaks can't be computed).
    """
    mode = partnership.get("frequency_mode")
    if mode == "interval":
        return partnership.get("frequency_interval_days")
    if mode == "weekly":
        return 7
    return None


def compute_streaks(
    session_timestamps: list[datetime], cycle_days: int, now: datetime | None = None
) -> Streaks:
    """Compute current/longest streaks of cycles with at least one logged session."""
    if not session_timestamps or cycle_days <= 0:
        return Streaks(current=0, longest=0)

    now = now if now is not None else utcnow()
    first = min(session_timestamps)
    cycles_met = {(ts - first).days // cycle_days for ts in session_timestamps}
    latest_cycle = max(0, (now - first).days // cycle_days)

    longest = 0
    run = 0
    for cycle in range(0, latest_cycle + 1):
        if cycle in cycles_met:
            run += 1
            longest = max(longest, run)
        else:
            run = 0

    # The in-progress cycle (latest_cycle) isn't over yet, so not having logged in
    # it doesn't break the streak — start counting from the last cycle that's
    # actually met, or from one cycle back if the current one is still pending.
    current = 0
    cycle = latest_cycle if latest_cycle in cycles_met else latest_cycle - 1
    while cycle >= 0 and cycle in cycles_met:
        current += 1
        cycle -= 1

    return Streaks(current=current, longest=longest)


def coverage_summary(sessions: list[dict[str, Any]]) -> list[tuple[datetime, str]]:
    """Chronological (logged_at, text_covered) pairs for a partnership's sessions."""
    return [(parse_timestamp(s["logged_at"]), s["text_covered"]) for s in sessions]
