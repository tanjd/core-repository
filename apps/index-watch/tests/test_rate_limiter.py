"""Tests for per-user command rate limiting."""

from datetime import UTC, datetime, timedelta

from index_watch.rate_limiter import RateLimiter


def test_check_rate_limit_first_call_passes() -> None:
    limiter = RateLimiter()
    assert limiter.check_rate_limit("user1", "daily", 60) is None


def test_check_rate_limit_immediate_repeat_blocked() -> None:
    limiter = RateLimiter()
    limiter.check_rate_limit("user1", "daily", 60)
    remaining = limiter.check_rate_limit("user1", "daily", 60)
    assert remaining is not None
    assert 0 < remaining <= 60


def test_check_rate_limit_different_commands_independent() -> None:
    limiter = RateLimiter()
    limiter.check_rate_limit("user1", "daily", 60)
    assert limiter.check_rate_limit("user1", "status", 10) is None


def test_check_rate_limit_different_users_independent() -> None:
    limiter = RateLimiter()
    limiter.check_rate_limit("user1", "daily", 60)
    assert limiter.check_rate_limit("user2", "daily", 60) is None


def test_check_rate_limit_passes_after_cooldown() -> None:
    limiter = RateLimiter()
    limiter._last_request["user1"]["daily"] = datetime.now(UTC) - timedelta(seconds=61)
    assert limiter.check_rate_limit("user1", "daily", 60) is None


def test_reset_user() -> None:
    limiter = RateLimiter()
    limiter.check_rate_limit("user1", "daily", 60)
    limiter.reset_user("user1")
    assert limiter.check_rate_limit("user1", "daily", 60) is None


def test_reset_user_not_present_is_noop() -> None:
    limiter = RateLimiter()
    limiter.reset_user("nope")  # must not raise


def test_cleanup_old_entries_removes_stale_user() -> None:
    limiter = RateLimiter()
    limiter._last_request["user1"]["daily"] = datetime.now(UTC) - timedelta(hours=25)
    limiter.cleanup_old_entries(max_age_hours=24)
    assert "user1" not in limiter._last_request


def test_cleanup_old_entries_keeps_recent_user() -> None:
    limiter = RateLimiter()
    limiter._last_request["user1"]["daily"] = datetime.now(UTC)
    limiter.cleanup_old_entries(max_age_hours=24)
    assert "user1" in limiter._last_request


def test_cleanup_old_entries_partial_cleanup_keeps_user_with_recent_command() -> None:
    limiter = RateLimiter()
    now = datetime.now(UTC)
    limiter._last_request["user1"]["daily"] = now - timedelta(hours=25)
    limiter._last_request["user1"]["status"] = now
    limiter.cleanup_old_entries(max_age_hours=24)
    assert "user1" in limiter._last_request
    assert "daily" not in limiter._last_request["user1"]
    assert "status" in limiter._last_request["user1"]
