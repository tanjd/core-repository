"""GET /api/holdings."""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import HoldingsResponse, HoldingsTotals, PositionItem
from app.models.db import Position, Statement

router = APIRouter()


@router.get("/holdings", response_model=HoldingsResponse)
def get_holdings(
    year: int | None = None,
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> HoldingsResponse:
    stmts = _get_statements(year, broker, session)
    stmt_ids = [s.id for s in stmts if s.id is not None]
    stmt_broker = {s.id: s.broker for s in stmts if s.id is not None}
    primary_year = stmts[0].year

    positions = session.exec(
        select(Position)
        .where(Position.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(Position.current_value.desc())  # type: ignore
    ).all()

    items = [
        PositionItem(
            symbol=p.symbol,
            description=p.description,
            isin=p.isin,
            quantity=p.quantity,
            cost_price=p.cost_price,
            cost_basis=p.cost_basis,
            close_price=p.close_price,
            current_value=p.current_value,
            unrealized_pnl=p.unrealized_pnl,
            broker=stmt_broker.get(p.statement_id, "ibkr"),
        )
        for p in positions
    ]

    totals = HoldingsTotals(
        cost_basis=sum(p.cost_basis for p in positions),
        current_value=sum(p.current_value for p in positions),
        unrealized_pnl=sum(p.unrealized_pnl for p in positions),
    )

    return HoldingsResponse(year=primary_year, positions=items, totals=totals)


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
