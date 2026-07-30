"""Tests for CNN Fear & Greed Index fetching, caching, and stale-fallback."""

from dataclasses import dataclass
from datetime import UTC, datetime, timedelta

import pytest

from index_watch.cache import get_cache
from index_watch.fear_greed import fetch_fear_greed


@pytest.fixture(autouse=True)
def _clear_cache():
    get_cache().clear()
    yield
    get_cache().clear()


@dataclass
class FakeFearGreed:
    value: float
    description: str
    last_update: str


def test_fetch_fear_greed_success(monkeypatch: pytest.MonkeyPatch) -> None:
    fake = FakeFearGreed(value=42.0, description="Neutral", last_update="2024-01-01")
    monkeypatch.setattr("fear_and_greed.get", lambda: fake)
    result = fetch_fear_greed()
    assert result is not None
    assert result.value == 42.0
    assert result.description == "Neutral"


def test_fetch_fear_greed_uses_cache_on_second_call(monkeypatch: pytest.MonkeyPatch) -> None:
    calls: list[int] = []

    def fake_get() -> FakeFearGreed:
        calls.append(1)
        return FakeFearGreed(value=10.0, description="Fear", last_update="x")

    monkeypatch.setattr("fear_and_greed.get", fake_get)
    fetch_fear_greed()
    fetch_fear_greed()
    assert len(calls) == 1


def test_fetch_fear_greed_failure_no_cache_returns_none(monkeypatch: pytest.MonkeyPatch) -> None:
    def raise_error() -> FakeFearGreed:
        raise RuntimeError("boom")

    monkeypatch.setattr("fear_and_greed.get", raise_error)
    assert fetch_fear_greed() is None


def test_fetch_fear_greed_failure_falls_back_to_stale_cache(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = FakeFearGreed(value=42.0, description="Neutral", last_update="2024-01-01")
    monkeypatch.setattr("fear_and_greed.get", lambda: fake)
    fetch_fear_greed()  # populate cache

    cache = get_cache()
    cached = cache._cache["fear_greed:latest"]
    cached.fetched_at = datetime.now(UTC) - timedelta(seconds=cached.ttl_seconds + 10)

    def raise_error() -> FakeFearGreed:
        raise RuntimeError("boom")

    monkeypatch.setattr("fear_and_greed.get", raise_error)
    result = fetch_fear_greed()
    assert result is not None
    assert result.value == 42.0
