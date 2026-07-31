"""Core ingest service — parse CSV → upsert all records to SQLite.

Called by both the watcher (background thread) and the upload API route.
Re-ingesting the same (account_id, year) is safe: existing records are
deleted and replaced, making the operation idempotent.
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from datetime import UTC, date, datetime
from pathlib import Path
from typing import Any

from sqlalchemy import delete as sa_delete
from sqlalchemy import func
from sqlmodel import Session, col, select

from app.models.db import (
    CorporateAction,
    Deposit,
    Dividend,
    Fee,
    MtmRecord,
    PnlRecord,
    Position,
    Statement,
    Trade,
    UploadLog,
    WithholdingTax,
)
from app.parser.base import parse_ibkr_csv
from app.parser.moomoo import (
    extract_moomoo_account_id,
    extract_moomoo_date,
    extract_moomoo_year,
    parse_moomoo_positions,
    parse_moomoo_trades,
)
from app.parser.sections.cash import parse_deposits
from app.parser.sections.income import parse_dividends, parse_fees, parse_withholding_tax
from app.parser.sections.instruments import build_instrument_info, parse_instruments
from app.parser.sections.nav import parse_nav
from app.parser.sections.performance import parse_corporate_actions, parse_mtm, parse_pnl
from app.parser.sections.positions import parse_positions
from app.parser.sections.statement import parse_statement_meta
from app.parser.sections.trades import parse_trades

logger = logging.getLogger(__name__)

_IBKR_RE = re.compile(r"^[A-Z]\d+_\d{4}(?:\d{4})?_\d{4}(?:\d{4})?\.csv$")


@dataclass
class IngestCounts:
    position_count: int = field(default=0)
    trade_count: int = field(default=0)
    deposit_count: int = field(default=0)
    dividend_count: int = field(default=0)


def write_upload_log(
    session: Session,
    *,
    filename: str,
    broker: str,
    account_id: str,
    account_name: str,
    year: int,
    period_end: date | None,
    nav_current: float,
    twr_pct: float,
    position_count: int,
    trade_count: int,
    deposit_count: int,
    dividend_count: int,
    source: str,
    status: str,
    error_msg: str | None = None,
) -> None:
    """Insert an UploadLog row. Failures are swallowed so they never break the main ingest."""
    try:
        log = UploadLog(
            filename=filename,
            broker=broker,
            account_id=account_id,
            account_name=account_name,
            year=year,
            period_end=period_end,
            nav_current=nav_current,
            twr_pct=twr_pct,
            position_count=position_count,
            trade_count=trade_count,
            deposit_count=deposit_count,
            dividend_count=dividend_count,
            source=source,
            status=status,
            error_msg=error_msg,
        )
        session.add(log)
        session.commit()
    except Exception:
        logger.exception("Failed to write upload log (non-fatal)")


_MOOMOO_TRADES_RE = re.compile(r"^History-Margin Account\(\d+\)-\d{8}-\d{6}\.csv$")
_MOOMOO_POSITIONS_RE = re.compile(r"^Positions-Margin Account\(\d+\)-\d{8}-\d{6}\.csv$")


def detect_broker(filename: str) -> str:
    """Return broker type from filename: 'ibkr' | 'moomoo-trades' | 'moomoo-positions'."""
    if _MOOMOO_TRADES_RE.match(filename):
        return "moomoo-trades"
    if _MOOMOO_POSITIONS_RE.match(filename):
        return "moomoo-positions"
    return "ibkr"


def ingest_file(filepath: str | Path, session: Session) -> tuple[Statement, IngestCounts]:
    """Parse a CSV (IBKR or moomoo) and upsert all records.

    Dispatches to the appropriate broker-specific ingestor based on filename.
    Returns the primary Statement (most recent year for moomoo trades) and IngestCounts.
    """
    broker = detect_broker(Path(filepath).name)
    if broker == "moomoo-trades":
        return ingest_moomoo_trades(filepath, session)
    if broker == "moomoo-positions":
        return ingest_moomoo_positions(filepath, session)
    return _ingest_ibkr(filepath, session)


def _ingest_ibkr(filepath: str | Path, session: Session) -> tuple[Statement, IngestCounts]:
    """Parse an IBKR CSV and upsert all records for that (account_id, year)."""
    logger.info("Ingesting %s", filepath)
    raw = parse_ibkr_csv(filepath)

    stmt_meta = parse_statement_meta(raw)
    account_id: str = stmt_meta["account_id"]
    year: int = stmt_meta["year"]

    alias_map = parse_instruments(raw)
    nav_data: dict[str, Any] = parse_nav(raw)
    instrument_info = build_instrument_info(raw, alias_map)

    _delete_existing(account_id, year, session)

    statement = Statement(
        account_id=account_id,
        account_name=stmt_meta["account_name"],
        year=year,
        period=stmt_meta["period"],
        period_start=stmt_meta["period_start"],
        period_end=stmt_meta["period_end"],
        broker="ibkr",
        base_currency=stmt_meta["base_currency"],
        twr_pct=nav_data["twr_pct"],
        nav_prior=nav_data["nav_prior"],
        nav_current=nav_data["nav_current"],
        nav_change=nav_data["nav_change"],
        nav_stock=nav_data["nav_stock"],
        nav_cash=nav_data["nav_cash"],
        starting_value=nav_data["starting_value"],
        ending_value=nav_data["ending_value"],
        deposits_withdrawals=nav_data["deposits_withdrawals"],
        mark_to_market=nav_data["mark_to_market"],
        dividends=nav_data["dividends"],
        withholding_tax=nav_data["withholding_tax"],
        commissions=nav_data["commissions"],
        other_fees=nav_data["other_fees"],
        sales_tax=nav_data["sales_tax"],
        ingested_at=datetime.now(UTC).replace(tzinfo=None),
    )
    session.add(statement)
    session.flush()  # Populate statement.id before inserting child rows

    assert statement.id is not None
    stmt_id = statement.id

    _ingest_positions(raw, stmt_id, year, alias_map, instrument_info, session)
    _ingest_trades(raw, stmt_id, year, alias_map, session)
    _ingest_deposits(raw, stmt_id, year, session)
    _ingest_dividends(raw, stmt_id, year, session)
    _ingest_withholding_tax(raw, stmt_id, year, session)
    _ingest_fees(raw, stmt_id, year, session)
    _ingest_pnl(raw, stmt_id, year, alias_map, session)
    _ingest_mtm(raw, stmt_id, year, alias_map, session)
    _ingest_corporate_actions(raw, stmt_id, year, session)

    session.commit()

    counts = IngestCounts(
        position_count=session.exec(
            select(func.count(col(Position.id))).where(Position.statement_id == stmt_id)
        ).one(),  # type: ignore[arg-type]
        trade_count=session.exec(
            select(func.count(col(Trade.id))).where(
                Trade.statement_id == stmt_id, Trade.asset_category != "Forex"
            )
        ).one(),  # type: ignore[arg-type]
        deposit_count=session.exec(
            select(func.count(col(Deposit.id))).where(Deposit.statement_id == stmt_id)
        ).one(),  # type: ignore[arg-type]
        dividend_count=session.exec(
            select(func.count(col(Dividend.id))).where(Dividend.statement_id == stmt_id)
        ).one(),  # type: ignore[arg-type]
    )
    logger.info("Ingested year=%d account=%s (statement.id=%d)", year, account_id, stmt_id)
    return statement, counts


# ---------------------------------------------------------------------------
# Moomoo ingestors
# ---------------------------------------------------------------------------


def ingest_moomoo_trades(filepath: str | Path, session: Session) -> tuple[Statement, IngestCounts]:
    """Parse a moomoo trade history CSV and upsert Trade records per calendar year.

    Creates one Statement per year found in the file.  Existing Trade rows for
    each affected (account_id, year) statement are deleted and replaced; Position
    rows are left untouched (partial upsert).
    Returns the Statement for the most recent year and combined IngestCounts.
    """
    filepath = Path(filepath)
    logger.info("Ingesting moomoo trades: %s", filepath)
    trades_by_year = parse_moomoo_trades(filepath)
    account_id = extract_moomoo_account_id(filepath)
    export_date = extract_moomoo_date(filepath)

    last_stmt: Statement | None = None
    total_trades = 0
    for year in sorted(trades_by_year):
        stmt = _get_or_create_moomoo_statement(account_id, year, session, export_date)
        assert stmt.id is not None
        # Partial delete: trades only
        session.execute(sa_delete(Trade).where(Trade.statement_id == stmt.id))  # type: ignore
        session.flush()
        for t in trades_by_year[year]:
            session.add(
                Trade(
                    statement_id=stmt.id,
                    year=year,
                    asset_category=t["asset_category"],
                    currency=t["currency"],
                    symbol=t["symbol"],
                    trade_date=t["trade_date"],
                    quantity=t["quantity"],
                    trade_price=t["trade_price"],
                    close_price=t["close_price"],
                    proceeds=t["proceeds"],
                    commission=t["commission"],
                    basis=t["basis"],
                    realized_pnl=t["realized_pnl"],
                    mtm_pnl=t["mtm_pnl"],
                    codes=t["codes"],
                    direction=t["direction"],
                )
            )
        total_trades += len(trades_by_year[year])
        last_stmt = stmt
        logger.info(
            "Ingested %d moomoo trades for year=%d account=%s",
            len(trades_by_year[year]),
            year,
            account_id,
        )

    session.commit()
    if last_stmt is None:
        raise ValueError("No filled trades found in moomoo trade history file")
    counts = IngestCounts(trade_count=total_trades)
    return last_stmt, counts


def ingest_moomoo_positions(
    filepath: str | Path, session: Session
) -> tuple[Statement, IngestCounts]:
    """Parse a moomoo positions snapshot CSV and upsert Position records.

    Creates or updates the Statement for the snapshot year.  Existing Position
    rows are deleted and replaced; Trade rows are left untouched (partial upsert).
    Also updates nav_current / nav_stock on the Statement.
    """
    filepath = Path(filepath)
    logger.info("Ingesting moomoo positions: %s", filepath)
    positions = parse_moomoo_positions(filepath)
    account_id = extract_moomoo_account_id(filepath)
    year = extract_moomoo_year(filepath)
    export_date = extract_moomoo_date(filepath)

    stmt = _get_or_create_moomoo_statement(account_id, year, session, export_date)
    assert stmt.id is not None

    # Update NAV from positions market values
    nav_current = sum(p["current_value"] for p in positions)
    stmt.nav_current = nav_current
    stmt.nav_stock = nav_current
    session.add(stmt)

    # Partial delete: positions only
    session.execute(sa_delete(Position).where(Position.statement_id == stmt.id))  # type: ignore
    session.flush()

    for p in positions:
        session.add(
            Position(
                statement_id=stmt.id,
                year=year,
                symbol=p["symbol"],
                description=p["description"],
                isin=p["isin"],
                asset_category=p["asset_category"],
                currency=p["currency"],
                quantity=p["quantity"],
                cost_price=p["cost_price"],
                cost_basis=p["cost_basis"],
                close_price=p["close_price"],
                current_value=p["current_value"],
                unrealized_pnl=p["unrealized_pnl"],
            )
        )

    session.commit()
    logger.info(
        "Ingested %d moomoo positions for year=%d account=%s", len(positions), year, account_id
    )
    counts = IngestCounts(position_count=len(positions))
    return stmt, counts


def _get_or_create_moomoo_statement(
    account_id: str, year: int, session: Session, export_date: date | None = None
) -> Statement:
    """Fetch existing moomoo Statement or create a minimal one.

    If export_date is provided and more recent than the stored period_end, updates it.
    """
    existing = session.exec(
        select(Statement).where(Statement.account_id == account_id, Statement.year == year)
    ).first()
    if existing is not None:
        if export_date is not None and (
            existing.period_end is None or export_date > existing.period_end
        ):
            existing.period_end = export_date
            session.add(existing)
            session.flush()
        return existing

    stmt = Statement(
        account_id=account_id,
        account_name=f"Moomoo {account_id}",
        year=year,
        period=str(year),
        broker="moomoo",
        base_currency="USD",
        period_end=export_date,
        ingested_at=datetime.now(UTC).replace(tzinfo=None),
    )
    session.add(stmt)
    session.flush()
    return stmt


# ---------------------------------------------------------------------------
# Delete
# ---------------------------------------------------------------------------


def _delete_existing(account_id: str, year: int, session: Session) -> None:
    existing = session.exec(
        select(Statement).where(Statement.account_id == account_id, Statement.year == year)
    ).first()
    if existing is None:
        return
    stmt_id = existing.id
    assert stmt_id is not None
    session.execute(sa_delete(Position).where(Position.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(Trade).where(Trade.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(Deposit).where(Deposit.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(Dividend).where(Dividend.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(WithholdingTax).where(WithholdingTax.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(Fee).where(Fee.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(PnlRecord).where(PnlRecord.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(MtmRecord).where(MtmRecord.statement_id == stmt_id))  # type: ignore
    session.execute(sa_delete(CorporateAction).where(CorporateAction.statement_id == stmt_id))  # type: ignore
    session.delete(existing)
    session.flush()


# ---------------------------------------------------------------------------
# Insert helpers
# ---------------------------------------------------------------------------


def _ingest_positions(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    alias_map: dict[str, str],
    instrument_info: dict[str, dict[str, str]],
    session: Session,
) -> None:
    for pos in parse_positions(raw, alias_map):
        info = instrument_info.get(pos["symbol"], {})
        session.add(
            Position(
                statement_id=stmt_id,
                year=year,
                symbol=pos["symbol"],
                description=info.get("description", ""),
                isin=info.get("isin") or None,
                asset_category=pos["asset_category"],
                currency=pos["currency"],
                quantity=pos["quantity"],
                cost_price=pos["cost_price"],
                cost_basis=pos["cost_basis"],
                close_price=pos["close_price"],
                current_value=pos["current_value"],
                unrealized_pnl=pos["unrealized_pnl"],
            )
        )


def _ingest_trades(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    alias_map: dict[str, str],
    session: Session,
) -> None:
    for t in parse_trades(raw, alias_map):
        session.add(
            Trade(
                statement_id=stmt_id,
                year=year,
                asset_category=t["asset_category"],
                currency=t["currency"],
                symbol=t["symbol"],
                trade_date=t["trade_date"],
                quantity=t["quantity"],
                trade_price=t["trade_price"],
                close_price=t["close_price"],
                proceeds=t["proceeds"],
                commission=t["commission"],
                basis=t["basis"],
                realized_pnl=t["realized_pnl"],
                mtm_pnl=t["mtm_pnl"],
                codes=t["codes"],
                direction=t["direction"],
            )
        )


def _ingest_deposits(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    session: Session,
) -> None:
    for d in parse_deposits(raw):
        session.add(
            Deposit(
                statement_id=stmt_id,
                year=year,
                settle_date=d["settle_date"],
                currency=d["currency"],
                description=d["description"],
                amount=d["amount"],
            )
        )


def _ingest_dividends(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    session: Session,
) -> None:
    for d in parse_dividends(raw):
        session.add(
            Dividend(
                statement_id=stmt_id,
                year=year,
                pay_date=d["pay_date"],
                currency=d["currency"],
                symbol=d["symbol"],
                isin=d["isin"] or None,
                per_share_rate=d["per_share_rate"],
                gross_amount=d["gross_amount"],
                dividend_type=d["dividend_type"],
                description=d["description"],
            )
        )


def _ingest_withholding_tax(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    session: Session,
) -> None:
    for t in parse_withholding_tax(raw):
        session.add(
            WithholdingTax(
                statement_id=stmt_id,
                year=year,
                tax_date=t["tax_date"],
                currency=t["currency"],
                symbol=t["symbol"],
                isin=t["isin"] or None,
                amount=t["amount"],
                description=t["description"],
            )
        )


def _ingest_fees(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    session: Session,
) -> None:
    for f in parse_fees(raw):
        session.add(
            Fee(
                statement_id=stmt_id,
                year=year,
                fee_date=f["fee_date"],
                currency=f["currency"],
                description=f["description"],
                amount=f["amount"],
                fee_type=f["fee_type"],
            )
        )


def _ingest_pnl(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    alias_map: dict[str, str],
    session: Session,
) -> None:
    for p in parse_pnl(raw, alias_map):
        session.add(
            PnlRecord(
                statement_id=stmt_id,
                year=year,
                symbol=p["symbol"],
                asset_category=p["asset_category"],
                realized_st_profit=p["realized_st_profit"],
                realized_st_loss=p["realized_st_loss"],
                realized_lt_profit=p["realized_lt_profit"],
                realized_lt_loss=p["realized_lt_loss"],
                realized_total=p["realized_total"],
                unrealized_st_profit=p["unrealized_st_profit"],
                unrealized_st_loss=p["unrealized_st_loss"],
                unrealized_lt_profit=p["unrealized_lt_profit"],
                unrealized_lt_loss=p["unrealized_lt_loss"],
                unrealized_total=p["unrealized_total"],
                total=p["total"],
            )
        )


def _ingest_mtm(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    alias_map: dict[str, str],
    session: Session,
) -> None:
    for m in parse_mtm(raw, alias_map):
        session.add(
            MtmRecord(
                statement_id=stmt_id,
                year=year,
                symbol=m["symbol"],
                asset_category=m["asset_category"],
                prior_qty=m["prior_qty"],
                current_qty=m["current_qty"],
                prior_price=m["prior_price"],
                current_price=m["current_price"],
                mtm_position=m["mtm_position"],
                mtm_transaction=m["mtm_transaction"],
                mtm_commissions=m["mtm_commissions"],
                mtm_other=m["mtm_other"],
                mtm_total=m["mtm_total"],
            )
        )


def _ingest_corporate_actions(
    raw: dict[str, Any],
    stmt_id: int,
    year: int,
    session: Session,
) -> None:
    for ca in parse_corporate_actions(raw):
        session.add(
            CorporateAction(
                statement_id=stmt_id,
                year=year,
                action_date=ca["action_date"],
                currency=ca.get("currency", ""),
                symbol=ca["symbol"],
                isin=ca["isin"] or None,
                description=ca["description"],
                quantity=ca["quantity"],
                proceeds=ca["proceeds"],
                value=ca["value"],
                realized_pnl=ca["realized_pnl"],
            )
        )
