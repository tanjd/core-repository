"""SQLModel table definitions — one row per ingested record."""

from __future__ import annotations

from datetime import UTC, date, datetime
from typing import ClassVar

from sqlalchemy import UniqueConstraint
from sqlmodel import Field, SQLModel


class Statement(SQLModel, table=True):
    """One row per (account_id, year).  All child tables FK here."""

    __tablename__: ClassVar[str] = "statement"  # type: ignore
    __table_args__: ClassVar[tuple[UniqueConstraint]] = (UniqueConstraint("account_id", "year"),)

    id: int | None = Field(default=None, primary_key=True)
    account_id: str
    account_name: str
    year: int
    period: str
    # Date range of the statement — None for legacy rows ingested before this field was added.
    # YTD example: period_start=2026-01-01, period_end=2026-03-02
    # Annual example: period_start=2025-01-01, period_end=2025-12-31
    period_start: date | None = None
    period_end: date | None = None
    # Source broker — "ibkr" for Interactive Brokers; future: "moomoo", etc.
    broker: str = "ibkr"
    base_currency: str = "USD"
    # NAV
    twr_pct: float = 0.0
    nav_prior: float = 0.0
    nav_current: float = 0.0
    nav_change: float = 0.0
    nav_stock: float = 0.0
    nav_cash: float = 0.0
    # Change in NAV
    starting_value: float = 0.0
    ending_value: float = 0.0
    deposits_withdrawals: float = 0.0
    mark_to_market: float = 0.0
    dividends: float = 0.0
    withholding_tax: float = 0.0
    commissions: float = 0.0
    other_fees: float = 0.0
    sales_tax: float = 0.0
    ingested_at: datetime = Field(default_factory=lambda: datetime.now(UTC).replace(tzinfo=None))


class Position(SQLModel, table=True):
    """Open position as of end of the statement year."""

    __tablename__: ClassVar[str] = "position"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    symbol: str
    description: str = ""
    isin: str | None = None
    asset_category: str = "Stocks"
    currency: str = "USD"
    quantity: float = 0.0
    cost_price: float = 0.0
    cost_basis: float = 0.0
    close_price: float = 0.0
    current_value: float = 0.0
    unrealized_pnl: float = 0.0


class Trade(SQLModel, table=True):
    """Individual executed order — stocks or forex."""

    __tablename__: ClassVar[str] = "trade"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    asset_category: str  # "Stocks" | "Forex"
    currency: str
    symbol: str
    trade_date: datetime
    quantity: float = 0.0
    trade_price: float = 0.0
    close_price: float = 0.0
    proceeds: float = 0.0
    commission: float = 0.0
    basis: float = 0.0
    realized_pnl: float = 0.0
    mtm_pnl: float = 0.0
    codes: str = "[]"  # JSON-encoded list e.g. '["O","RI"]'
    direction: str = "buy"  # "buy" | "sell"


class Deposit(SQLModel, table=True):
    """Deposit or withdrawal transaction."""

    __tablename__: ClassVar[str] = "deposit"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    settle_date: date
    currency: str
    description: str = ""
    amount: float = 0.0


class Dividend(SQLModel, table=True):
    """Cash dividend payment."""

    __tablename__: ClassVar[str] = "dividend"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    pay_date: date
    currency: str
    symbol: str
    isin: str | None = None
    per_share_rate: float = 0.0
    gross_amount: float = 0.0
    dividend_type: str = ""
    description: str = ""


class WithholdingTax(SQLModel, table=True):
    """US withholding tax applied to dividends (including prior-year carry-overs)."""

    __tablename__: ClassVar[str] = "withholding_tax"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    tax_date: date
    currency: str
    symbol: str
    isin: str | None = None
    amount: float = 0.0
    description: str = ""


class Fee(SQLModel, table=True):
    """Fee, commission adjustment, or referral credit."""

    __tablename__: ClassVar[str] = "fee"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    fee_date: date
    currency: str
    description: str = ""
    amount: float = 0.0
    fee_type: str = ""  # "Other Fees" | "Commission Adjustment"


class PnlRecord(SQLModel, table=True):
    """Realized & unrealized P&L per symbol (short-term + long-term breakdown)."""

    __tablename__: ClassVar[str] = "pnl_record"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    symbol: str
    asset_category: str
    realized_st_profit: float = 0.0
    realized_st_loss: float = 0.0
    realized_lt_profit: float = 0.0
    realized_lt_loss: float = 0.0
    realized_total: float = 0.0
    unrealized_st_profit: float = 0.0
    unrealized_st_loss: float = 0.0
    unrealized_lt_profit: float = 0.0
    unrealized_lt_loss: float = 0.0
    unrealized_total: float = 0.0
    total: float = 0.0


class MtmRecord(SQLModel, table=True):
    """Mark-to-market performance per symbol."""

    __tablename__: ClassVar[str] = "mtm_record"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    symbol: str
    asset_category: str
    prior_qty: float = 0.0
    current_qty: float = 0.0
    prior_price: float = 0.0
    current_price: float = 0.0
    mtm_position: float = 0.0
    mtm_transaction: float = 0.0
    mtm_commissions: float = 0.0
    mtm_other: float = 0.0
    mtm_total: float = 0.0


class CorporateAction(SQLModel, table=True):
    """Stock splits, mergers, and other corporate events."""

    __tablename__: ClassVar[str] = "corporate_action"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    statement_id: int = Field(foreign_key="statement.id")
    year: int
    action_date: date
    currency: str = ""
    symbol: str
    isin: str | None = None
    description: str = ""
    quantity: float = 0.0
    proceeds: float = 0.0
    value: float = 0.0
    realized_pnl: float = 0.0


class UploadLog(SQLModel, table=True):
    """Append-only audit log of every ingest event (upload or watcher)."""

    __tablename__: ClassVar[str] = "upload_log"  # type: ignore

    id: int | None = Field(default=None, primary_key=True)
    uploaded_at: datetime = Field(default_factory=lambda: datetime.now(UTC).replace(tzinfo=None))
    filename: str
    broker: str = "unknown"
    account_id: str = ""
    account_name: str = ""
    year: int = 0
    period_end: date | None = None
    nav_current: float = 0.0
    twr_pct: float = 0.0
    position_count: int = 0
    trade_count: int = 0
    deposit_count: int = 0
    dividend_count: int = 0
    source: str = "upload"  # "upload" | "watcher"
    status: str = "success"  # "success" | "error"
    error_msg: str | None = None
