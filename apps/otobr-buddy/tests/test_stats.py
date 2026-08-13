from datetime import datetime, timedelta

from otobr_buddy import stats


def _dt(days_offset: int) -> datetime:
    return datetime(2026, 1, 1) + timedelta(days=days_offset)


def test_cycle_length_days_interval():
    partnership = {"frequency_mode": "interval", "frequency_interval_days": 5}
    assert stats.cycle_length_days(partnership) == 5


def test_cycle_length_days_weekly():
    partnership = {"frequency_mode": "weekly"}
    assert stats.cycle_length_days(partnership) == 7


def test_cycle_length_days_unset():
    assert stats.cycle_length_days({}) is None


def test_compute_streaks_no_sessions():
    result = stats.compute_streaks([], 7)
    assert result.current == 0
    assert result.longest == 0


def test_compute_streaks_perfect_run():
    timestamps = [_dt(0), _dt(7), _dt(14)]
    result = stats.compute_streaks(timestamps, 7, now=_dt(15))
    assert result.current == 3
    assert result.longest == 3


def test_compute_streaks_broken_streak():
    # Cycle 0 met, cycle 1 missed, cycle 2 met.
    timestamps = [_dt(0), _dt(14)]
    result = stats.compute_streaks(timestamps, 7, now=_dt(15))
    assert result.current == 1
    assert result.longest == 1


def test_compute_streaks_current_cycle_in_progress_not_penalized():
    timestamps = [_dt(0), _dt(7)]
    result = stats.compute_streaks(timestamps, 7, now=_dt(15))
    assert result.current == 2


def test_coverage_summary_orders_pairs():
    sessions = [
        {"logged_at": "2026-01-01 00:00:00", "text_covered": "Romans 1"},
        {"logged_at": "2026-01-08 00:00:00", "text_covered": "Romans 2"},
    ]
    coverage = stats.coverage_summary(sessions)
    assert coverage == [
        (datetime(2026, 1, 1), "Romans 1"),
        (datetime(2026, 1, 8), "Romans 2"),
    ]
