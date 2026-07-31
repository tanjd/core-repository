"""GET /api/years and GET /api/overview."""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import (
    AssetAllocation,
    BrokerInfo,
    BrokerNAV,
    ChangeInNav,
    NavSummary,
    OverviewResponse,
)
from app.models.db import Position, Statement

router = APIRouter()


@router.get("/years", response_model=list[int])
def get_years(session: Session = Depends(get_session)) -> list[int]:
    statements = session.exec(select(Statement).order_by(Statement.year)).all()  # type: ignore
    return sorted({s.year for s in statements})


@router.get("/brokers", response_model=list[str])
def get_brokers(session: Session = Depends(get_session)) -> list[str]:
    """Return sorted list of distinct broker names present in the database."""
    statements = session.exec(select(Statement)).all()
    return sorted({s.broker for s in statements})


@router.get("/broker-info", response_model=list[BrokerInfo])
def get_broker_info(session: Session = Depends(get_session)) -> list[BrokerInfo]:
    """Return per-broker metadata: available years and latest statement period end date."""
    statements = session.exec(select(Statement)).all()
    brokers: dict[str, BrokerInfo] = {}
    for s in statements:
        if s.broker not in brokers:
            brokers[s.broker] = BrokerInfo(
                broker=s.broker, years=[], latest_period_end=s.period_end
            )
        else:
            if s.period_end is not None:
                current = brokers[s.broker].latest_period_end
                if current is None or s.period_end > current:
                    brokers[s.broker].latest_period_end = s.period_end
        brokers[s.broker].years.append(s.year)
    for info in brokers.values():
        info.years.sort()
    return sorted(brokers.values(), key=lambda b: b.broker)


@router.get("/overview", response_model=OverviewResponse)
def get_overview(
    year: int | None = None,
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> OverviewResponse:
    stmts = _get_statements(year, broker, session)

    # Use the IBKR statement for period/change-in-nav metadata; fall back to first
    primary = next((s for s in stmts if s.broker == "ibkr"), stmts[0])

    # Aggregate NAV across all brokers
    nav_current = sum(s.nav_current for s in stmts)
    nav_prior = sum(s.nav_prior for s in stmts)
    nav_change = sum(s.nav_change for s in stmts)

    # Asset allocation from positions across all brokers
    stmt_ids = [s.id for s in stmts if s.id is not None]
    positions = session.exec(
        select(Position).where(Position.statement_id.in_(stmt_ids))  # type: ignore
    ).all()
    stock_value = sum(p.current_value for p in positions if p.asset_category == "Stocks")
    ibkr_stmt = next((s for s in stmts if s.broker == "ibkr"), None)
    cash_value = ibkr_stmt.nav_cash if ibkr_stmt else 0.0

    asset_allocation = []
    if stock_value:
        asset_allocation.append(AssetAllocation(asset_class="Stock", value=stock_value))
    if cash_value:
        asset_allocation.append(AssetAllocation(asset_class="Cash", value=cash_value))

    broker_breakdown = [
        BrokerNAV(broker=s.broker, currency=s.base_currency, nav_current=s.nav_current)
        for s in stmts
    ]

    return OverviewResponse(
        year=primary.year,
        period=primary.period,
        account_id=primary.account_id,
        account_name=primary.account_name,
        nav=NavSummary(
            prior_total=nav_prior,
            current_total=nav_current,
            change=nav_change,
            twr_pct=primary.twr_pct,
        ),
        change_in_nav=ChangeInNav(
            starting_value=primary.starting_value,
            ending_value=primary.ending_value,
            deposits_withdrawals=primary.deposits_withdrawals,
            mark_to_market=primary.mark_to_market,
            dividends=primary.dividends,
            withholding_tax=primary.withholding_tax,
            commissions=primary.commissions,
            other_fees=primary.other_fees,
            sales_tax=primary.sales_tax,
        ),
        asset_allocation=asset_allocation,
        broker_breakdown=broker_breakdown,
    )


def _get_statements(year: int | None, broker: str | None, session: Session) -> list[Statement]:
    if year is not None:
        query = select(Statement).where(Statement.year == year)  # type: ignore
    else:
        latest = session.exec(
            select(Statement).order_by(Statement.year.desc())  # type: ignore
        ).first()
        if latest is None:
            raise HTTPException(status_code=404, detail="No data found for the requested year")
        query = select(Statement).where(Statement.year == latest.year)  # type: ignore

    if broker:
        query = query.where(Statement.broker == broker)  # type: ignore

    stmts = session.exec(query).all()
    if not stmts:
        raise HTTPException(status_code=404, detail="No data found for the requested year")
    return list(stmts)
