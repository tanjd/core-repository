"""GET /api/timeseries/* — multi-year aggregate endpoints."""

from __future__ import annotations

from collections import defaultdict

from fastapi import APIRouter, Depends
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import (
    CommissionTimeseriesItem,
    DcaItem,
    DepositTimeseriesItem,
    DividendTimeseriesItem,
    NavTimeseriesItem,
    PnlTimeseriesItem,
    PositionTimeseriesItem,
)
from app.models.db import (
    Deposit,
    Dividend,
    Fee,
    PnlRecord,
    Position,
    Statement,
    Trade,
    WithholdingTax,
)

router = APIRouter()


@router.get("/timeseries/nav", response_model=list[NavTimeseriesItem])
def timeseries_nav(
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> list[NavTimeseriesItem]:
    query = select(Statement).order_by(Statement.year)  # type: ignore
    if broker:
        query = query.where(Statement.broker == broker)  # type: ignore
    statements = session.exec(query).all()

    # Aggregate NAV across brokers per year; use IBKR twr_pct where available
    nav_by_year: dict[int, dict[str, float]] = {}
    for s in statements:
        if s.year not in nav_by_year:
            nav_by_year[s.year] = {"nav_prior": 0.0, "nav_current": 0.0, "twr_pct": 0.0}
        nav_by_year[s.year]["nav_prior"] += s.nav_prior
        nav_by_year[s.year]["nav_current"] += s.nav_current
        if s.broker == "ibkr":
            nav_by_year[s.year]["twr_pct"] = s.twr_pct

    return [
        NavTimeseriesItem(
            year=year,
            nav_prior=vals["nav_prior"],
            nav_current=vals["nav_current"],
            twr_pct=vals["twr_pct"],
        )
        for year, vals in sorted(nav_by_year.items())
    ]


@router.get("/timeseries/deposits", response_model=list[DepositTimeseriesItem])
def timeseries_deposits(
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> list[DepositTimeseriesItem]:
    query = select(Statement).order_by(Statement.year)  # type: ignore
    if broker:
        query = query.where(Statement.broker == broker)  # type: ignore
    statements = session.exec(query).all()

    deposits_by_year: dict[int, float] = defaultdict(float)

    # IBKR: use actual cash deposits recorded in the statement
    for s in statements:
        if s.broker == "ibkr":
            deposits_by_year[s.year] += s.deposits_withdrawals

    # Moomoo has no deposit export — approximate net invested from trade proceeds.
    # proceeds convention: negative for buys, positive for sells.
    # -sum(proceeds) = total buy cost − total sell proceeds = net capital deployed.
    moomoo_stmt_ids = {
        s.id: s.year for s in statements if s.broker == "moomoo" and s.id is not None
    }
    if moomoo_stmt_ids:
        for trade in session.exec(
            select(Trade).where(Trade.statement_id.in_(list(moomoo_stmt_ids)))  # type: ignore
        ).all():
            year = moomoo_stmt_ids.get(trade.statement_id)
            if year is not None:
                deposits_by_year[year] += -trade.proceeds

    cumulative = 0.0
    result = []
    for year in sorted(deposits_by_year):
        cumulative += deposits_by_year[year]
        result.append(
            DepositTimeseriesItem(
                year=year,
                total_deposits=deposits_by_year[year],
                cumulative_deposits=cumulative,
            )
        )
    return result


@router.get("/timeseries/dividends", response_model=list[DividendTimeseriesItem])
def timeseries_dividends(
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> list[DividendTimeseriesItem]:
    query = select(Statement).order_by(Statement.year)  # type: ignore
    if broker:
        query = query.where(Statement.broker == broker)  # type: ignore
    statements = session.exec(query).all()
    stmt_ids = {s.id: s.year for s in statements if s.id is not None}
    unique_years = sorted({s.year for s in statements})

    gross_by_year: dict[int, float] = defaultdict(float)
    for d in session.exec(select(Dividend)).all():
        year = stmt_ids.get(d.statement_id)
        if year is not None:
            gross_by_year[year] += d.gross_amount

    withholding_by_year: dict[int, float] = defaultdict(float)
    for t in session.exec(select(WithholdingTax)).all():
        year = stmt_ids.get(t.statement_id)
        if year is not None:
            withholding_by_year[year] += t.amount

    fees_by_year: dict[int, float] = defaultdict(float)
    for f in session.exec(select(Fee)).all():
        if f.fee_type == "Commission Adjustment":
            continue
        year = stmt_ids.get(f.statement_id)
        if year is not None:
            fees_by_year[year] += f.amount

    return [
        DividendTimeseriesItem(
            year=year,
            gross=gross_by_year[year],
            withholding=withholding_by_year[year],
            net=gross_by_year[year] + withholding_by_year[year],
            fees=fees_by_year[year],
        )
        for year in unique_years
    ]


@router.get("/timeseries/pnl", response_model=list[PnlTimeseriesItem])
def timeseries_pnl(
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> list[PnlTimeseriesItem]:
    query = select(Statement).order_by(Statement.year)  # type: ignore
    if broker:
        query = query.where(Statement.broker == broker)  # type: ignore
    statements = session.exec(query).all()
    stmt_ids = {s.id: s.year for s in statements if s.id is not None}
    unique_years = sorted({s.year for s in statements})

    realized_by_year: dict[int, float] = defaultdict(float)
    unrealized_by_year: dict[int, float] = defaultdict(float)

    for r in session.exec(select(PnlRecord)).all():
        year = stmt_ids.get(r.statement_id)
        if year is not None:
            realized_by_year[year] += r.realized_total
            unrealized_by_year[year] += r.unrealized_total

    return [
        PnlTimeseriesItem(
            year=year,
            realized=realized_by_year[year],
            unrealized=unrealized_by_year[year],
        )
        for year in unique_years
    ]


@router.get("/timeseries/dca", response_model=list[DcaItem])
def timeseries_dca(session: Session = Depends(get_session)) -> list[DcaItem]:
    """Monthly deposit amounts across all years (proxy for DCA pattern).

    Returns SGD and USD totals separately per (year, month) so the
    frontend can show both currencies without lossy FX conversion.
    """
    deposits = session.exec(select(Deposit).order_by(Deposit.settle_date)).all()  # type: ignore

    sgd_monthly: dict[tuple[int, int], float] = defaultdict(float)
    usd_monthly: dict[tuple[int, int], float] = defaultdict(float)
    for d in deposits:
        key = (d.settle_date.year, d.settle_date.month)
        if d.currency == "SGD":
            sgd_monthly[key] += d.amount
        elif d.currency == "USD":
            usd_monthly[key] += d.amount

    all_keys = sorted(sgd_monthly.keys() | usd_monthly.keys())
    return [
        DcaItem(
            year=year,
            month=month,
            sgd=sgd_monthly[(year, month)],
            usd=usd_monthly[(year, month)],
        )
        for year, month in all_keys
    ]


@router.get("/timeseries/positions", response_model=list[PositionTimeseriesItem])
def timeseries_positions(
    session: Session = Depends(get_session),
) -> list[PositionTimeseriesItem]:
    positions = session.exec(select(Position).order_by(Position.year, Position.symbol)).all()  # type: ignore

    return [
        PositionTimeseriesItem(
            year=p.year,
            symbol=p.symbol,
            quantity=p.quantity,
            unrealized_pnl=p.unrealized_pnl,
        )
        for p in positions
    ]


@router.get("/timeseries/commissions", response_model=list[CommissionTimeseriesItem])
def timeseries_commissions(
    session: Session = Depends(get_session),
) -> list[CommissionTimeseriesItem]:
    statements = session.exec(select(Statement).order_by(Statement.year)).all()  # type: ignore
    stmt_ids = {s.id: s.year for s in statements if s.id is not None}

    stocks_by_year: dict[int, float] = defaultdict(float)
    forex_by_year: dict[int, float] = defaultdict(float)

    for t in session.exec(select(Trade)).all():
        year = stmt_ids.get(t.statement_id)
        if year is not None:
            if t.asset_category == "Stocks":
                stocks_by_year[year] += t.commission
            elif t.asset_category == "Forex":
                forex_by_year[year] += t.commission

    unique_years = sorted({s.year for s in statements})
    return [
        CommissionTimeseriesItem(
            year=year,
            stocks=stocks_by_year[year],
            forex=forex_by_year[year],
            total=stocks_by_year[year] + forex_by_year[year],
        )
        for year in unique_years
    ]
