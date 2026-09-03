"""Shared pytest fixtures — a fresh app + in-memory-per-file SQLite DB per test."""

from __future__ import annotations

import importlib
from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client(tmp_path, monkeypatch) -> Iterator[TestClient]:
    """A TestClient wired to a throwaway SQLite DB and data dir per test.

    Reimports app.config/app.database/app.main so the engine picks up the env vars set here
    instead of whatever was configured at collection time.
    """
    data_dir = tmp_path / "data"
    data_dir.mkdir()
    monkeypatch.setenv("LEDGER_DATA_DIR", str(data_dir))
    monkeypatch.setenv("LEDGER_DATABASE_URL", f"sqlite:///{tmp_path}/test.db")

    import app.config
    import app.database
    import app.main

    importlib.reload(app.config)
    importlib.reload(app.database)
    main = importlib.reload(app.main)

    with TestClient(main.app) as c:
        yield c
