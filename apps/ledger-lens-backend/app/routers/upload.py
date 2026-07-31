"""Upload + preview routes for IBKR CSV statements and moomoo exports.

POST /api/preview  — parse CSV, return summary; no DB writes
POST /api/upload   — parse CSV, save to data/<broker>/, persist to DB
"""

from __future__ import annotations

import re
import tempfile
from pathlib import Path

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile
from sqlmodel import Session, col, select

from app.config import settings
from app.database import get_session
from app.models.api import PreviewResponse, UploadLogItem, UploadResponse
from app.models.db import Statement, UploadLog
from app.parser.base import parse_ibkr_csv
from app.parser.moomoo import (
    extract_moomoo_account_id,
    extract_moomoo_year,
    parse_moomoo_positions,
    parse_moomoo_trades,
)
from app.parser.sections.cash import parse_deposits
from app.parser.sections.income import parse_dividends
from app.parser.sections.instruments import parse_instruments
from app.parser.sections.nav import parse_nav
from app.parser.sections.positions import parse_positions
from app.parser.sections.statement import parse_statement_meta
from app.parser.sections.trades import parse_trades
from app.services.ingestor import detect_broker, ingest_file, write_upload_log

router = APIRouter()

# Filename patterns for supported brokers
_IBKR_RE = re.compile(r"^[A-Z]\d+_\d{4}(?:\d{4})?_\d{4}(?:\d{4})?\.csv$")
_MOOMOO_TRADES_RE = re.compile(r"^History-Margin Account\(\d+\)-\d{8}-\d{6}\.csv$")
_MOOMOO_POSITIONS_RE = re.compile(r"^Positions-Margin Account\(\d+\)-\d{8}-\d{6}\.csv$")


def _validate_filename(filename: str | None) -> str:
    name = filename or ""
    if _IBKR_RE.match(name) or _MOOMOO_TRADES_RE.match(name) or _MOOMOO_POSITIONS_RE.match(name):
        return name
    raise HTTPException(
        status_code=400,
        detail=(
            "Filename must match a supported format:\n"
            "  IBKR annual:  <AccountID>_<Year>_<Year>.csv (e.g. U11111111_2025_2025.csv)\n"
            "  IBKR YTD:     <AccountID>_<YYYYMMDD>_<YYYYMMDD>.csv\n"
            "  Moomoo trades:    History-Margin Account(<ID>)-<YYYYMMDD>-<HHMMSS>.csv\n"
            "  Moomoo positions: Positions-Margin Account(<ID>)-<YYYYMMDD>-<HHMMSS>.csv"
        ),
    )


def _dest_dir(broker_type: str) -> Path:
    """Return the broker-specific subdirectory under data_dir."""
    sub = "moomoo" if broker_type.startswith("moomoo") else "ibkr"
    dest = Path(settings.data_dir) / sub
    dest.mkdir(parents=True, exist_ok=True)
    return dest


@router.post("/preview", response_model=PreviewResponse)
async def preview_statement(
    file: UploadFile = File(...),
    session: Session = Depends(get_session),
) -> PreviewResponse:
    """Parse a CSV and return a summary without writing to the database."""
    name = _validate_filename(file.filename)
    broker_type = detect_broker(name)
    content = await file.read()

    with tempfile.NamedTemporaryFile(suffix=".csv", delete=False) as tmp:
        tmp.write(content)
        tmp_path = tmp.name

    try:
        if broker_type == "moomoo-trades":
            return _preview_moomoo_trades(tmp_path, name, session)
        if broker_type == "moomoo-positions":
            return _preview_moomoo_positions(tmp_path, name, session)
        return _preview_ibkr(tmp_path, session)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=422, detail=f"Failed to parse CSV: {exc}") from exc
    finally:
        Path(tmp_path).unlink(missing_ok=True)


def _preview_ibkr(tmp_path: str, session: Session) -> PreviewResponse:
    raw = parse_ibkr_csv(tmp_path)
    stmt_meta = parse_statement_meta(raw)
    nav_data = parse_nav(raw)
    alias_map = parse_instruments(raw)

    account_id: str = stmt_meta["account_id"]
    year: int = stmt_meta["year"]

    positions = parse_positions(raw, alias_map)
    stock_trades = [t for t in parse_trades(raw, alias_map) if t["asset_category"] != "Forex"]
    deposits = parse_deposits(raw)
    dividends = parse_dividends(raw)

    already_imported = (
        session.exec(
            select(Statement).where(Statement.account_id == account_id, Statement.year == year)
        ).first()
        is not None
    )

    period_end = stmt_meta.get("period_end")
    period_end_label = period_end.strftime("%B %-d, %Y") if period_end else str(year)

    return PreviewResponse(
        broker="ibkr",
        account_id=account_id,
        account_name=stmt_meta["account_name"],
        year=year,
        period=stmt_meta["period"],
        period_end_label=period_end_label,
        base_currency=stmt_meta["base_currency"],
        nav_current=nav_data["nav_current"],
        twr_pct=nav_data["twr_pct"],
        position_count=len(positions),
        trade_count=len(stock_trades),
        deposit_count=len(deposits),
        dividend_count=len(dividends),
        already_imported=already_imported,
    )


def _preview_moomoo_trades(tmp_path: str, filename: str, session: Session) -> PreviewResponse:
    fp = Path(tmp_path)
    fp_named = fp.with_name(filename)  # rename so helpers can parse account ID
    fp.rename(fp_named)
    try:
        trades_by_year = parse_moomoo_trades(fp_named)
        account_id = extract_moomoo_account_id(Path(filename))
    finally:
        # rename back to tmp_path so caller can unlink
        fp_named.rename(fp)

    total_trades = sum(len(v) for v in trades_by_year.values())
    years = sorted(trades_by_year)
    most_recent_year = years[-1] if years else 0

    already_imported = (
        session.exec(
            select(Statement).where(
                Statement.account_id == account_id,
                Statement.year == most_recent_year,
                Statement.broker == "moomoo",
            )
        ).first()
        is not None
    )

    return PreviewResponse(
        broker="moomoo",
        account_id=account_id,
        account_name=f"Moomoo {account_id}",
        year=most_recent_year,
        period=f"{years[0]}–{years[-1]}" if len(years) > 1 else str(most_recent_year),
        period_end_label=str(most_recent_year),
        base_currency="USD",
        nav_current=0.0,
        twr_pct=0.0,
        position_count=0,
        trade_count=total_trades,
        deposit_count=0,
        dividend_count=0,
        already_imported=already_imported,
        years_detected=years,
    )


def _preview_moomoo_positions(tmp_path: str, filename: str, session: Session) -> PreviewResponse:
    fp = Path(tmp_path)
    fp_named = fp.with_name(filename)
    fp.rename(fp_named)
    try:
        positions = parse_moomoo_positions(fp_named)
        account_id = extract_moomoo_account_id(Path(filename))
        year = extract_moomoo_year(Path(filename))
    finally:
        fp_named.rename(fp)

    nav_current = sum(p["current_value"] for p in positions)

    already_imported = (
        session.exec(
            select(Statement).where(
                Statement.account_id == account_id,
                Statement.year == year,
                Statement.broker == "moomoo",
            )
        ).first()
        is not None
    )

    return PreviewResponse(
        broker="moomoo",
        account_id=account_id,
        account_name=f"Moomoo {account_id}",
        year=year,
        period=str(year),
        period_end_label=str(year),
        base_currency="USD",
        nav_current=nav_current,
        twr_pct=0.0,
        position_count=len(positions),
        trade_count=0,
        deposit_count=0,
        dividend_count=0,
        already_imported=already_imported,
        years_detected=[year],
    )


@router.post("/upload", response_model=UploadResponse)
async def upload_statement(
    file: UploadFile = File(...),
    session: Session = Depends(get_session),
) -> UploadResponse:
    """Save a CSV to data/<broker>/ and ingest it into the database."""
    name = _validate_filename(file.filename)
    broker_type = detect_broker(name)

    dest_dir = _dest_dir(broker_type)
    dest = dest_dir / name

    content = await file.read()
    dest.write_bytes(content)

    try:
        statement, counts = ingest_file(str(dest), session)
    except Exception as exc:
        write_upload_log(
            session,
            filename=name,
            broker=broker_type,
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
            source="upload",
            status="error",
            error_msg=str(exc),
        )
        raise HTTPException(status_code=422, detail=str(exc)) from exc

    end_label = (
        statement.period_end.strftime("%B %-d, %Y") if statement.period_end else str(statement.year)
    )
    write_upload_log(
        session,
        filename=name,
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
        source="upload",
        status="success",
    )
    return UploadResponse(
        year=statement.year,
        period=statement.period,
        period_end_label=end_label,
        account_id=statement.account_id,
        broker=statement.broker,
        account_name=statement.account_name,
        position_count=counts.position_count,
        trade_count=counts.trade_count,
        deposit_count=counts.deposit_count,
        dividend_count=counts.dividend_count,
        nav_current=statement.nav_current,
        twr_pct=statement.twr_pct,
    )


@router.get("/upload-history", response_model=list[UploadLogItem])
def get_upload_history(session: Session = Depends(get_session)) -> list[UploadLogItem]:
    """Return all upload log entries, newest first."""
    logs = session.exec(select(UploadLog).order_by(col(UploadLog.uploaded_at).desc())).all()
    return [UploadLogItem.model_validate(log, from_attributes=True) for log in logs]
