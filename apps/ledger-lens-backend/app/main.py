"""FastAPI application entry point."""

from __future__ import annotations

import json
import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.config import settings
from app.database import create_all_tables, engine, migrate_schema
from app.routers import (
    cashflows,
    holdings,
    income,
    overview,
    performance,
    timeseries,
    trades,
    upload,
)
from app.services.migration import migrate_data_dir
from app.services.watcher import ingest_existing, start_watcher

logging.basicConfig(level=logging.INFO)


def _read_version() -> str:
    # nx release only ever bumps package.json, not pyproject.toml — read the
    # version from the file nx release actually writes.
    try:
        data = json.loads((Path(__file__).parent.parent / "package.json").read_text())
        return data["version"]
    except FileNotFoundError, KeyError:
        return "dev"


_VERSION = _read_version()


@asynccontextmanager
async def lifespan(_app: FastAPI) -> AsyncGenerator[None]:
    # SQLite can't create the db file inside a directory that doesn't exist yet —
    # on a fresh checkout data_dir has never been created (Docker's image does this
    # via the Dockerfile instead, so this only matters for local dev).
    Path(settings.data_dir).mkdir(parents=True, exist_ok=True)
    migrate_schema(engine)
    create_all_tables()
    migrate_data_dir(settings.data_dir)
    ingest_existing(settings.data_dir)
    observer = start_watcher(settings.data_dir)
    yield
    observer.stop()
    observer.join()


app = FastAPI(title="Ledger Lens", version=_VERSION, lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(upload.router, prefix="/api")
app.include_router(overview.router, prefix="/api")
app.include_router(holdings.router, prefix="/api")
app.include_router(trades.router, prefix="/api")
app.include_router(income.router, prefix="/api")
app.include_router(cashflows.router, prefix="/api")
app.include_router(performance.router, prefix="/api")
app.include_router(timeseries.router, prefix="/api")


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/api/version")
def get_version() -> dict[str, str]:
    return {"version": _VERSION}
