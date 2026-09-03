"""GET /api/returns/xirr — money-weighted return, presented as annualized personal return.

v1 is scoped to a single broker (default "ibkr") because only IBKR statements carry real,
individually-dated cash flows (see app/parser/sections/cash.py) — Moomoo's deposits are an
annual trade-proceeds *approximation*, not dated per transaction, and mixing the two would
produce a number that looks exactly as precise as a real XIRR while being partly guessed.
"""

from __future__ import annotations

from datetime import date

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import XirrTimeseriesItem
from app.models.db import Deposit, Statement
from app.services.returns import xirr

router = APIRouter()


@router.get("/returns/xirr", response_model=list[XirrTimeseriesItem])
def get_xirr_timeseries(
    broker: str = "ibkr",
    session: Session = Depends(get_session),
) -> list[XirrTimeseriesItem]:
    statements = session.exec(
        select(Statement).where(Statement.broker == broker).order_by(Statement.year)  # type: ignore
    ).all()
    if not statements:
        raise HTTPException(status_code=404, detail=f"No statements found for broker={broker}")

    currencies = {s.base_currency for s in statements}
    if len(currencies) > 1:
        raise HTTPException(
            status_code=409,
            detail=f"Cannot compute XIRR across mixed currencies: {sorted(currencies)}",
        )

    stmt_ids = [s.id for s in statements if s.id is not None]
    deposits = session.exec(
        select(Deposit)
        .where(Deposit.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(Deposit.settle_date)  # type: ignore
    ).all()

    deposit_currencies = {d.currency for d in deposits}
    if len(deposit_currencies) > 1 or (deposit_currencies and deposit_currencies != currencies):
        raise HTTPException(
            status_code=409,
            detail=(
                "Cannot compute XIRR across mixed currencies: "
                f"{sorted(deposit_currencies | currencies)}"
            ),
        )

    result: list[XirrTimeseriesItem] = []
    for s in statements:
        as_of = s.period_end or date(s.year, 12, 31)
        cash_flows = [(d.settle_date, -d.amount) for d in deposits if d.settle_date <= as_of]
        cash_flows.append((as_of, s.nav_current))
        rate = xirr(cash_flows)
        result.append(
            XirrTimeseriesItem(
                year=s.year,
                as_of_date=as_of,
                xirr_pct=rate,
                cash_flow_count=len(cash_flows) - 1,
            )
        )
    return result
