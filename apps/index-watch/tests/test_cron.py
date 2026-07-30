"""Tests for the daily-report cron string parser."""

import logging

import pytest

from index_watch.bot import _cron_from_cronstr


def test_cron_from_cronstr_full_spec() -> None:
    assert _cron_from_cronstr("0 22 * * 1-5") == {
        "minute": "0",
        "hour": "22",
        "day_of_week": "1-5",
    }


def test_cron_from_cronstr_all_wildcards() -> None:
    assert _cron_from_cronstr("* * * * *") == {}


def test_cron_from_cronstr_malformed_field_count_falls_back_and_warns(
    caplog: pytest.LogCaptureFixture,
) -> None:
    caplog.set_level(logging.WARNING)
    result = _cron_from_cronstr("0 22 * *")
    assert result == {}
    assert "malformed" in caplog.text.lower()


def test_cron_from_cronstr_empty_string_falls_back_and_warns(
    caplog: pytest.LogCaptureFixture,
) -> None:
    caplog.set_level(logging.WARNING)
    result = _cron_from_cronstr("")
    assert result == {}
    assert "malformed" in caplog.text.lower()
