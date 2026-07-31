// TypeScript interfaces mirroring the backend Pydantic response schemas.

// ---------------------------------------------------------------------------
// Overview
// ---------------------------------------------------------------------------

export interface NavSummary {
  prior_total: number;
  current_total: number;
  change: number;
  twr_pct: number;
}

export interface ChangeInNav {
  starting_value: number;
  ending_value: number;
  deposits_withdrawals: number;
  mark_to_market: number;
  dividends: number;
  withholding_tax: number;
  commissions: number;
  other_fees: number;
  sales_tax: number;
}

export interface AssetAllocation {
  asset_class: string;
  value: number;
}

export interface BrokerNAV {
  broker: string;
  currency: string;
  nav_current: number;
}

export interface OverviewResponse {
  year: number;
  period: string;
  account_id: string;
  account_name: string;
  nav: NavSummary;
  change_in_nav: ChangeInNav;
  asset_allocation: AssetAllocation[];
  broker_breakdown: BrokerNAV[];
}

// ---------------------------------------------------------------------------
// Holdings
// ---------------------------------------------------------------------------

export interface PositionItem {
  symbol: string;
  description: string;
  isin: string | null;
  quantity: number;
  cost_price: number;
  cost_basis: number;
  close_price: number;
  current_value: number;
  unrealized_pnl: number;
  broker: string;
}

export interface HoldingsTotals {
  cost_basis: number;
  current_value: number;
  unrealized_pnl: number;
}

export interface HoldingsResponse {
  year: number;
  positions: PositionItem[];
  totals: HoldingsTotals;
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

export interface TradeItem {
  id: number;
  trade_date: string;
  symbol: string;
  currency: string;
  quantity: number;
  trade_price: number;
  proceeds: number;
  commission: number;
  basis: number;
  realized_pnl: number;
  mtm_pnl: number;
  codes: string[];
  direction: string;
  broker: string;
}

export interface TradesResponse {
  year: number;
  asset_type: string;
  trades: TradeItem[];
}

// ---------------------------------------------------------------------------
// Income
// ---------------------------------------------------------------------------

export interface DividendItem {
  pay_date: string;
  currency: string;
  symbol: string;
  isin: string | null;
  per_share_rate: number;
  gross_amount: number;
  dividend_type: string;
}

export interface WithholdingTaxItem {
  tax_date: string;
  currency: string;
  symbol: string;
  amount: number;
}

export interface FeeItem {
  fee_date: string;
  currency: string;
  description: string;
  amount: number;
  fee_type: string;
}

export interface IncomeSummary {
  gross_dividends: number;
  withholding_tax: number;
  net_dividends: number;
  fees: number;
  commission_adjustments: number;
}

export interface IncomeResponse {
  year: number;
  summary: IncomeSummary;
  dividends: DividendItem[];
  withholding_tax: WithholdingTaxItem[];
  fees: FeeItem[];
}

// ---------------------------------------------------------------------------
// Cash Flows
// ---------------------------------------------------------------------------

export interface DepositItem {
  settle_date: string;
  currency: string;
  description: string;
  amount: number;
}

export interface MonthlyCashflow {
  month: string;
  sgd: number;
  usd: number;
}

export interface CashflowsResponse {
  year: number;
  deposits: DepositItem[];
  by_currency: Record<string, number>;
  total_usd: number;
  monthly: MonthlyCashflow[];
}

// ---------------------------------------------------------------------------
// Performance (P&L)
// ---------------------------------------------------------------------------

export interface PnlItem {
  symbol: string;
  asset_category: string;
  realized_st_profit: number;
  realized_st_loss: number;
  realized_lt_profit: number;
  realized_lt_loss: number;
  realized_total: number;
  unrealized_st_profit: number;
  unrealized_st_loss: number;
  unrealized_lt_profit: number;
  unrealized_lt_loss: number;
  unrealized_total: number;
  total: number;
}

export interface MtmItem {
  symbol: string;
  asset_category: string;
  prior_qty: number;
  current_qty: number;
  prior_price: number;
  current_price: number;
  mtm_position: number;
  mtm_transaction: number;
  mtm_commissions: number;
  mtm_other: number;
  mtm_total: number;
}

export interface CorporateActionItem {
  action_date: string;
  symbol: string;
  description: string;
  quantity: number;
  proceeds: number;
  value: number;
  realized_pnl: number;
}

export interface PnlSummary {
  realized_st: number;
  realized_lt: number;
  realized_total: number;
  unrealized_total: number;
  total: number;
}

export interface PerformanceResponse {
  year: number;
  pnl_records: PnlItem[];
  summary: PnlSummary;
  mtm_summary: MtmItem[];
  corporate_actions: CorporateActionItem[];
}

// ---------------------------------------------------------------------------
// Timeseries (multi-year)
// ---------------------------------------------------------------------------

export interface NavTimeseriesItem {
  year: number;
  nav_prior: number;
  nav_current: number;
  twr_pct: number;
}

export interface DepositTimeseriesItem {
  year: number;
  total_deposits: number;
  cumulative_deposits: number;
}

export interface DividendTimeseriesItem {
  year: number;
  gross: number;
  withholding: number;
  net: number;
  fees: number;
}

export interface PnlTimeseriesItem {
  year: number;
  realized: number;
  unrealized: number;
}

export interface DcaItem {
  year: number;
  month: number;
  sgd: number;
  usd: number;
}

export interface PositionTimeseriesItem {
  year: number;
  symbol: string;
  quantity: number;
  unrealized_pnl: number;
}

export interface CommissionTimeseriesItem {
  year: number;
  stocks: number;
  forex: number;
  total: number;
}

// ---------------------------------------------------------------------------
// Upload / Preview
// ---------------------------------------------------------------------------

export interface PreviewResponse {
  broker: string;
  account_id: string;
  account_name: string;
  year: number;
  period: string;
  /** Human-readable end date, e.g. "March 2, 2026" (YTD) or "December 31, 2025" (annual) */
  period_end_label: string;
  base_currency: string;
  nav_current: number;
  twr_pct: number;
  position_count: number;
  trade_count: number;
  deposit_count: number;
  dividend_count: number;
  already_imported: boolean;
  years_detected?: number[];
}

export interface UploadResponse {
  year: number;
  period: string;
  period_end_label: string;
  account_id: string;
  broker: string;
  account_name: string;
  position_count: number;
  trade_count: number;
  deposit_count: number;
  dividend_count: number;
  nav_current: number;
  twr_pct: number;
}

export interface UploadLogItem {
  id: number;
  uploaded_at: string;
  filename: string;
  broker: string;
  account_id: string;
  account_name: string;
  year: number;
  period_end: string | null;
  nav_current: number;
  twr_pct: number;
  position_count: number;
  trade_count: number;
  deposit_count: number;
  dividend_count: number;
  source: string;
  status: string;
  error_msg: string | null;
}

// ---------------------------------------------------------------------------
// Version
// ---------------------------------------------------------------------------

export interface VersionResponse {
  version: string;
}

export interface BrokerInfo {
  broker: string;
  years: number[];
  latest_period_end: string | null; // ISO date e.g. "2025-12-31"
}
