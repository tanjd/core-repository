"""Shared pytest fixtures for index-watch tests."""

import pytest

from index_watch import database


@pytest.fixture(autouse=True)
def _temp_db(tmp_path, monkeypatch):
    """Point database.py at an isolated per-test SQLite file."""
    monkeypatch.setattr(database, "DB_PATH", tmp_path / "test.db")
    database.init_db()
    yield
