"""Watchdog file-system observer for the data/ directory.

Automatically ingests any new or modified *.csv file dropped into data/.
Started and stopped via the FastAPI lifespan context manager in main.py.

Debounce: rapid-fire events for the same file (e.g. on_created + on_modified)
are coalesced into a single ingest after a 1.5 s quiet window.
"""

from __future__ import annotations

import logging
import threading
from pathlib import Path

from watchdog.events import FileSystemEvent, FileSystemEventHandler
from watchdog.observers import Observer
from watchdog.observers.api import BaseObserver

from app.database import engine
from app.services.ingestor import ingest_file, write_upload_log

logger = logging.getLogger(__name__)

_DEBOUNCE_SECONDS = 1.5


class _CsvHandler(FileSystemEventHandler):
    def __init__(self) -> None:
        super().__init__()
        self._timers: dict[str, threading.Timer] = {}
        self._lock = threading.Lock()

    def on_created(self, event: FileSystemEvent) -> None:
        if not event.is_directory and str(event.src_path).endswith(".csv"):
            self._schedule(str(event.src_path))

    def on_modified(self, event: FileSystemEvent) -> None:
        if not event.is_directory and str(event.src_path).endswith(".csv"):
            self._schedule(str(event.src_path))

    def _schedule(self, filepath: str) -> None:
        with self._lock:
            existing = self._timers.pop(filepath, None)
            if existing is not None:
                existing.cancel()
            timer = threading.Timer(_DEBOUNCE_SECONDS, self._try_ingest, args=[filepath])
            self._timers[filepath] = timer
            timer.start()

    def _try_ingest(self, filepath: str) -> None:
        from sqlmodel import Session

        with self._lock:
            self._timers.pop(filepath, None)

        logger.info("Watcher detected change: %s", filepath)
        filename = Path(filepath).name
        try:
            with Session(engine) as session:
                statement, counts = ingest_file(filepath, session)
                write_upload_log(
                    session,
                    filename=filename,
                    broker=statement.broker,
                    account_id=statement.account_id,
                    account_name=statement.account_name,
                    year=statement.year,
                    period_end=statement.period_end,
                    nav_current=statement.nav_current,
                    twr_pct=statement.twr_pct,
                    position_count=counts.position_count,
                    trade_count=counts.trade_count,
                    deposit_count=counts.deposit_count,
                    dividend_count=counts.dividend_count,
                    source="watcher",
                    status="success",
                )
        except Exception as exc:
            logger.exception("Watcher failed to ingest %s", filepath)
            try:
                with Session(engine) as session:
                    write_upload_log(
                        session,
                        filename=filename,
                        broker="unknown",
                        account_id="",
                        account_name="",
                        year=0,
                        period_end=None,
                        nav_current=0.0,
                        twr_pct=0.0,
                        position_count=0,
                        trade_count=0,
                        deposit_count=0,
                        dividend_count=0,
                        source="watcher",
                        status="error",
                        error_msg=str(exc),
                    )
            except Exception:
                logger.exception("Failed to write error upload log for %s", filepath)


def start_watcher(data_dir: str) -> BaseObserver:
    observer: BaseObserver = Observer()
    observer.schedule(_CsvHandler(), data_dir, recursive=True)
    observer.start()
    logger.info("Watching %s for new CSV files", data_dir)
    return observer


def _already_ingested(filename: str) -> bool:
    """Return True if a successful UploadLog entry exists for this filename."""
    from sqlmodel import Session, select

    from app.models.db import UploadLog

    with Session(engine) as session:
        return (
            session.exec(
                select(UploadLog)
                .where(UploadLog.filename == filename, UploadLog.status == "success")
                .limit(1)
            ).first()
            is not None
        )


def ingest_existing(data_dir: str) -> None:
    """Ingest all *.csv files already present in data_dir at startup."""
    from sqlmodel import Session

    for csv_file in Path(data_dir).glob("**/*.csv"):
        filename = csv_file.name
        if _already_ingested(filename):
            logger.debug("Startup ingest: skipping already-ingested %s", filename)
            continue
        logger.info("Startup ingest: %s", csv_file)
        try:
            with Session(engine) as session:
                statement, counts = ingest_file(str(csv_file), session)
                write_upload_log(
                    session,
                    filename=filename,
                    broker=statement.broker,
                    account_id=statement.account_id,
                    account_name=statement.account_name,
                    year=statement.year,
                    period_end=statement.period_end,
                    nav_current=statement.nav_current,
                    twr_pct=statement.twr_pct,
                    position_count=counts.position_count,
                    trade_count=counts.trade_count,
                    deposit_count=counts.deposit_count,
                    dividend_count=counts.dividend_count,
                    source="watcher",
                    status="success",
                )
        except Exception as exc:
            logger.exception("Startup ingest failed for %s", csv_file)
            try:
                with Session(engine) as session:
                    write_upload_log(
                        session,
                        filename=filename,
                        broker="unknown",
                        account_id="",
                        account_name="",
                        year=0,
                        period_end=None,
                        nav_current=0.0,
                        twr_pct=0.0,
                        position_count=0,
                        trade_count=0,
                        deposit_count=0,
                        dividend_count=0,
                        source="watcher",
                        status="error",
                        error_msg=str(exc),
                    )
            except Exception:
                logger.exception("Failed to write error upload log for %s", csv_file)
