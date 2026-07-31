"""SQLModel engine and session factory."""

from __future__ import annotations

from sqlalchemy import Engine, text
from sqlmodel import Session, SQLModel, create_engine

from app.config import settings

engine = create_engine(
    settings.db_url,
    # Required for SQLite in a multi-threaded context (watchdog runs in a background thread)
    connect_args={"check_same_thread": False},
)


def migrate_schema(eng: Engine) -> None:
    """Add columns introduced after the initial schema — safe to run on every startup.

    SQLite does not support ALTER TABLE … ADD COLUMN IF NOT EXISTS, so we catch the
    OperationalError that fires when the column already exists.
    """
    new_cols = [
        "period_start DATE",
        "period_end DATE",
        "broker TEXT DEFAULT 'ibkr'",
    ]
    with eng.connect() as conn:
        for col_def in new_cols:
            try:
                _ = conn.execute(text(f"ALTER TABLE statement ADD COLUMN {col_def}"))
                conn.commit()
            except Exception:
                pass  # column already present — nothing to do


def create_all_tables() -> None:
    SQLModel.metadata.create_all(engine)


def get_session():
    with Session(engine) as session:
        yield session
