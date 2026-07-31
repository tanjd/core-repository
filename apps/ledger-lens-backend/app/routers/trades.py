"""GET /api/trades."""

from __future__ import annotations

import json

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import TradeItem, TradesResponse
from app.models.db import Statement, Trade

router = APIRouter()


@router.get("/trades", response_model=TradesResponse)
def get_trades(
    year: int | None = None,
    type: str = "stock",  # noqa: A002
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> TradesResponse:
    stmts = _get_statements(year, broker, session)
    stmt_ids = [s.id for s in stmts if s.id is not None]
    stmt_broker = {s.id: s.broker for s in stmts if s.id is not None}
    primary_year = stmts[0].year

    asset_category = "Stocks" if type == "stock" else "Forex"

    trades = session.exec(
        select(Trade)
        .where(Trade.statement_id.in_(stmt_ids), Trade.asset_category == asset_category)  # type: ignore
        .order_by(Trade.trade_date)  # type: ignore
    ).all()

    items = [
        TradeItem(
            id=t.id or 0,
            trade_date=t.trade_date,
            symbol=t.symbol,
            currency=t.currency,
            quantity=t.quantity,
            trade_price=t.trade_price,
            proceeds=t.proceeds,
            commission=t.commission,
            basis=t.basis,
            realized_pnl=t.realized_pnl,
            mtm_pnl=t.mtm_pnl,
            codes=_decode_codes(t.codes),
            direction=t.direction,
            broker=stmt_broker.get(t.statement_id, "ibkr"),
        )
        for t in trades
    ]

    return TradesResponse(year=primary_year, asset_type=type, trades=items)


def _decode_codes(codes_json: str) -> list[str]:
    try:
        result = json.loads(codes_json)
        return result if isinstance(result, list) else []
    except ValueError, TypeError:
        return []


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
