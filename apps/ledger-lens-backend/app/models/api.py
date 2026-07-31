"""Pydantic response schemas — the API contract with the frontend."""

from __future__ import annotations

from datetime import date, datetime
from typing import ClassVar

from pydantic import BaseModel, ConfigDict


class _Base(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(from_attributes=True)


# ---------------------------------------------------------------------------
# Broker info
# ---------------------------------------------------------------------------


class BrokerInfo(_Base):
    broker: str
    years: list[int]
    latest_period_end: date | None


# ---------------------------------------------------------------------------
# Overview
# ---------------------------------------------------------------------------


class NavSummary(_Base):
    prior_total: float
    current_total: float
    change: float
    twr_pct: float


class ChangeInNav(_Base):
    starting_value: float
    ending_value: float
    deposits_withdrawals: float
    mark_to_market: float
    dividends: float
    withholding_tax: float
    commissions: float
    other_fees: float
    sales_tax: float


class AssetAllocation(_Base):
    asset_class: str
    value: float


class BrokerNAV(_Base):
    broker: str
    currency: str
    nav_current: float


class OverviewResponse(_Base):
    year: int
    period: str
    account_id: str
    account_name: str
    nav: NavSummary
    change_in_nav: ChangeInNav
    asset_allocation: list[AssetAllocation]
    broker_breakdown: list[BrokerNAV] = []


# ---------------------------------------------------------------------------
# Holdings
# ---------------------------------------------------------------------------


class PositionItem(_Base):
    symbol: str
    description: str
    isin: str | None
    quantity: float
    cost_price: float
    cost_basis: float
    close_price: float
    current_value: float
    unrealized_pnl: float
    broker: str = "ibkr"


class HoldingsTotals(_Base):
    cost_basis: float
    current_value: float
    unrealized_pnl: float


class HoldingsResponse(_Base):
    year: int
    positions: list[PositionItem]
    totals: HoldingsTotals


# ---------------------------------------------------------------------------
# Trades
# ---------------------------------------------------------------------------


class TradeItem(_Base):
    id: int
    trade_date: datetime
    symbol: str
    currency: str
    quantity: float
    trade_price: float
    proceeds: float
    commission: float
    basis: float
    realized_pnl: float
    mtm_pnl: float
    codes: list[str]
    direction: str
    broker: str = "ibkr"


class TradesResponse(_Base):
    year: int
    asset_type: str  # "stock" | "forex"
    trades: list[TradeItem]


# ---------------------------------------------------------------------------
# Income
# ---------------------------------------------------------------------------


class DividendItem(_Base):
    pay_date: date
    currency: str
    symbol: str
    isin: str | None
    per_share_rate: float
    gross_amount: float
    dividend_type: str


class WithholdingTaxItem(_Base):
    tax_date: date
    currency: str
    symbol: str
    amount: float


class FeeItem(_Base):
    fee_date: date
    currency: str
    description: str
    amount: float
    fee_type: str


class IncomeSummary(_Base):
    gross_dividends: float
    withholding_tax: float
    net_dividends: float
    fees: float
    commission_adjustments: float


class IncomeResponse(_Base):
    year: int
    summary: IncomeSummary
    dividends: list[DividendItem]
    withholding_tax: list[WithholdingTaxItem]
    fees: list[FeeItem]


# ---------------------------------------------------------------------------
# Cash flows
# ---------------------------------------------------------------------------


class DepositItem(_Base):
    settle_date: date
    currency: str
    description: str
    amount: float


class MonthlyCashflow(_Base):
    month: str  # "2025-01"
    sgd: float
    usd: float


class CashflowsResponse(_Base):
    year: int
    deposits: list[DepositItem]
    by_currency: dict[str, float]
    total_usd: float
    monthly: list[MonthlyCashflow]


# ---------------------------------------------------------------------------
# Performance (P&L)
# ---------------------------------------------------------------------------


class PnlItem(_Base):
    symbol: str
    asset_category: str
    realized_st_profit: float
    realized_st_loss: float
    realized_lt_profit: float
    realized_lt_loss: float
    realized_total: float
    unrealized_st_profit: float
    unrealized_st_loss: float
    unrealized_lt_profit: float
    unrealized_lt_loss: float
    unrealized_total: float
    total: float


class MtmItem(_Base):
    symbol: str
    asset_category: str
    prior_qty: float
    current_qty: float
    prior_price: float
    current_price: float
    mtm_position: float
    mtm_transaction: float
    mtm_commissions: float
    mtm_other: float
    mtm_total: float


class CorporateActionItem(_Base):
    action_date: date
    symbol: str
    description: str
    quantity: float
    proceeds: float
    value: float
    realized_pnl: float


class PnlSummary(_Base):
    realized_st: float
    realized_lt: float
    realized_total: float
    unrealized_total: float
    total: float


class PerformanceResponse(_Base):
    year: int
    pnl_records: list[PnlItem]
    summary: PnlSummary
    mtm_summary: list[MtmItem]
    corporate_actions: list[CorporateActionItem]


# ---------------------------------------------------------------------------
# Timeseries (multi-year)
# ---------------------------------------------------------------------------


class NavTimeseriesItem(_Base):
    year: int
    nav_prior: float
    nav_current: float
    twr_pct: float


class DepositTimeseriesItem(_Base):
    year: int
    total_deposits: float
    cumulative_deposits: float


class DividendTimeseriesItem(_Base):
    year: int
    gross: float
    withholding: float
    net: float
    fees: float


class PnlTimeseriesItem(_Base):
    year: int
    realized: float
    unrealized: float


class DcaItem(_Base):
    year: int
    month: int
    sgd: float
    usd: float


class PositionTimeseriesItem(_Base):
    year: int
    symbol: str
    quantity: float
    unrealized_pnl: float


class CommissionTimeseriesItem(_Base):
    year: int
    stocks: float
    forex: float
    total: float


# ---------------------------------------------------------------------------
# Upload / Preview
# ---------------------------------------------------------------------------


class PreviewResponse(BaseModel):
    broker: str = "ibkr"
    account_id: str
    account_name: str
    year: int
    period: str
    # Human-readable end date: "March 2, 2026" (YTD) or "December 31, 2025" (annual)
    period_end_label: str
    base_currency: str
    nav_current: float
    twr_pct: float
    position_count: int
    trade_count: int
    deposit_count: int
    dividend_count: int
    already_imported: bool
    # For moomoo trade history files covering multiple years
    years_detected: list[int] | None = None


class UploadResponse(_Base):
    year: int
    period: str
    period_end_label: str
    account_id: str
    broker: str = "ibkr"
    account_name: str = ""
    position_count: int = 0
    trade_count: int = 0
    deposit_count: int = 0
    dividend_count: int = 0
    nav_current: float = 0.0
    twr_pct: float = 0.0


class UploadLogItem(BaseModel):
    id: int
    uploaded_at: datetime
    filename: str
    broker: str
    account_id: str
    account_name: str
    year: int
    period_end: date | None
    nav_current: float
    twr_pct: float
    position_count: int
    trade_count: int
    deposit_count: int
    dividend_count: int
    source: str
    status: str
    error_msg: str | None
