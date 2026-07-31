"""GET /api/cashflows."""

from __future__ import annotations

from collections import defaultdict

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import CashflowsResponse, DepositItem, MonthlyCashflow
from app.models.db import Deposit, Statement

router = APIRouter()


@router.get("/cashflows", response_model=CashflowsResponse)
def get_cashflows(
    year: int | None = None,
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> CashflowsResponse:
    stmts = _get_statements(year, broker, session)
    stmt_ids = [s.id for s in stmts if s.id is not None]
    primary_year = stmts[0].year
    total_usd = sum(s.deposits_withdrawals for s in stmts)

    deposits = session.exec(
        select(Deposit)
        .where(Deposit.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(Deposit.settle_date)  # type: ignore
    ).all()

    by_currency: dict[str, float] = defaultdict(float)
    monthly_raw: dict[str, dict[str, float]] = defaultdict(lambda: {"sgd": 0.0, "usd": 0.0})

    for d in deposits:
        by_currency[d.currency] += d.amount
        month_key = d.settle_date.strftime("%Y-%m")
        if d.currency == "SGD":
            monthly_raw[month_key]["sgd"] += d.amount
        elif d.currency == "USD":
            monthly_raw[month_key]["usd"] += d.amount

    monthly = [
        MonthlyCashflow(month=month, sgd=vals["sgd"], usd=vals["usd"])
        for month, vals in sorted(monthly_raw.items())
    ]

    return CashflowsResponse(
        year=primary_year,
        deposits=[
            DepositItem(
                settle_date=d.settle_date,
                currency=d.currency,
                description=d.description,
                amount=d.amount,
            )
            for d in deposits
        ],
        by_currency=dict(by_currency),
        total_usd=total_usd,
        monthly=monthly,
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
