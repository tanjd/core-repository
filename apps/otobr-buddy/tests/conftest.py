"""Shared pytest fixtures for otobr-buddy tests."""

import pytest

from otobr_buddy import database


@pytest.fixture(autouse=True)
def db(tmp_path):
    """Point database.py at an isolated per-test SQLite file."""
    database.set_db_path(tmp_path / "test.db")
    database.init_db()
    yield database
