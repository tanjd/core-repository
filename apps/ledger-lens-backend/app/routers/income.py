"""GET /api/income."""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException
from sqlmodel import Session, select

from app.database import get_session
from app.models.api import (
    DividendItem,
    FeeItem,
    IncomeResponse,
    IncomeSummary,
    WithholdingTaxItem,
)
from app.models.db import Dividend, Fee, Statement, WithholdingTax

router = APIRouter()


@router.get("/income", response_model=IncomeResponse)
def get_income(
    year: int | None = None,
    broker: str | None = None,
    session: Session = Depends(get_session),
) -> IncomeResponse:
    stmts = _get_statements(year, broker, session)
    stmt_ids = [s.id for s in stmts if s.id is not None]
    primary_year = stmts[0].year

    dividends = session.exec(
        select(Dividend)
        .where(Dividend.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(Dividend.pay_date)  # type: ignore
    ).all()

    withholding = session.exec(
        select(WithholdingTax)
        .where(WithholdingTax.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(WithholdingTax.tax_date)  # type: ignore
    ).all()

    fees = session.exec(
        select(Fee)
        .where(Fee.statement_id.in_(stmt_ids))  # type: ignore
        .order_by(Fee.fee_date)  # type: ignore
    ).all()

    gross_dividends = sum(d.gross_amount for d in dividends)
    withholding_total = sum(t.amount for t in withholding)
    fees_total = sum(f.amount for f in fees if f.fee_type not in ("Commission Adjustment",))
    comm_adj_total = sum(f.amount for f in fees if f.fee_type == "Commission Adjustment")

    return IncomeResponse(
        year=primary_year,
        summary=IncomeSummary(
            gross_dividends=gross_dividends,
            withholding_tax=withholding_total,
            net_dividends=gross_dividends + withholding_total,
            fees=fees_total,
            commission_adjustments=comm_adj_total,
        ),
        dividends=[
            DividendItem(
                pay_date=d.pay_date,
                currency=d.currency,
                symbol=d.symbol,
                isin=d.isin,
                per_share_rate=d.per_share_rate,
                gross_amount=d.gross_amount,
                dividend_type=d.dividend_type,
            )
            for d in dividends
        ],
        withholding_tax=[
            WithholdingTaxItem(
                tax_date=t.tax_date,
                currency=t.currency,
                symbol=t.symbol,
                amount=t.amount,
            )
            for t in withholding
        ],
        fees=[
            FeeItem(
                fee_date=f.fee_date,
                currency=f.currency,
                description=f.description,
                amount=f.amount,
                fee_type=f.fee_type,
            )
            for f in fees
        ],
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
