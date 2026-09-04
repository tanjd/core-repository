"use client";

import useSWR, { mutate as globalMutate } from "swr";
import {
  fetcher,
  BENCHMARKS_URL,
  BROKER_INFO_URL,
  BROKERS_URL,
  getBenchmarkTimeseriesUrl,
  getCashflowsUrl,
  getHoldingsUrl,
  getIncomeUrl,
  getOverviewUrl,
  getPerformanceUrl,
  getTimeseriesUrl,
  getTradesUrl,
  getXirrUrl,
  TIMESERIES_URLS,
  UPLOAD_HISTORY_URL,
  VERSION_URL,
} from "@/lib/api";
import type {
  BenchmarkOption,
  BenchmarkTimeseriesItem,
  BrokerInfo,
  CashflowsResponse,
  CommissionTimeseriesItem,
  DcaItem,
  DepositTimeseriesItem,
  DividendTimeseriesItem,
  HoldingsResponse,
  IncomeResponse,
  NavTimeseriesItem,
  OverviewResponse,
  PerformanceResponse,
  PnlTimeseriesItem,
  PositionTimeseriesItem,
  TradesResponse,
  UploadLogItem,
  VersionResponse,
  XirrTimeseriesItem,
} from "@/lib/types";

// ---------------------------------------------------------------------------
// Per-year hooks
// ---------------------------------------------------------------------------

export function useOverview(year: number | null, broker?: string) {
  const key = year ? getOverviewUrl(year, broker) : null;
  return useSWR<OverviewResponse>(key, fetcher);
}

export function useHoldings(year: number | null) {
  const key = year ? getHoldingsUrl(year) : null;
  return useSWR<HoldingsResponse>(key, fetcher);
}

export function useStockTrades(year: number | null) {
  const key = year ? getTradesUrl(year, "stock") : null;
  return useSWR<TradesResponse>(key, fetcher);
}

export function useForexTrades(year: number | null) {
  const key = year ? getTradesUrl(year, "forex") : null;
  return useSWR<TradesResponse>(key, fetcher);
}

export function useIncome(year: number | null) {
  const key = year ? getIncomeUrl(year) : null;
  return useSWR<IncomeResponse>(key, fetcher);
}

export function useCashflows(year: number | null) {
  const key = year ? getCashflowsUrl(year) : null;
  return useSWR<CashflowsResponse>(key, fetcher);
}

export function usePerformance(year: number | null) {
  const key = year ? getPerformanceUrl(year) : null;
  return useSWR<PerformanceResponse>(key, fetcher);
}

// ---------------------------------------------------------------------------
// Timeseries (multi-year) hooks
// ---------------------------------------------------------------------------

export function useNavTimeseries(broker?: string) {
  return useSWR<NavTimeseriesItem[]>(getTimeseriesUrl("nav", broker), fetcher);
}

export function useDepositTimeseries(broker?: string) {
  return useSWR<DepositTimeseriesItem[]>(
    getTimeseriesUrl("deposits", broker),
    fetcher,
  );
}

export function useDividendTimeseries(broker?: string) {
  return useSWR<DividendTimeseriesItem[]>(
    getTimeseriesUrl("dividends", broker),
    fetcher,
  );
}

export function usePnlTimeseries(broker?: string) {
  return useSWR<PnlTimeseriesItem[]>(getTimeseriesUrl("pnl", broker), fetcher);
}

export function useDcaTimeseries() {
  return useSWR<DcaItem[]>(TIMESERIES_URLS.dca, fetcher);
}

export function usePositionTimeseries() {
  return useSWR<PositionTimeseriesItem[]>(TIMESERIES_URLS.positions, fetcher);
}
export function useCommissionTimeseries() {
  return useSWR<CommissionTimeseriesItem[]>(
    TIMESERIES_URLS.commissions,
    fetcher,
  );
}

// ---------------------------------------------------------------------------
// Benchmark comparison / XIRR
// ---------------------------------------------------------------------------

export function useBenchmarks() {
  return useSWR<BenchmarkOption[]>(BENCHMARKS_URL, fetcher);
}

export function useBenchmarkTimeseries(symbol: string | null, broker?: string) {
  const key = symbol ? getBenchmarkTimeseriesUrl(symbol, broker) : null;
  return useSWR<BenchmarkTimeseriesItem[]>(key, fetcher);
}

export function useXirrTimeseries(broker?: string) {
  return useSWR<XirrTimeseriesItem[]>(getXirrUrl(broker), fetcher);
}

// ---------------------------------------------------------------------------
// App version
// ---------------------------------------------------------------------------

export function useBackendVersion() {
  return useSWR<VersionResponse>(VERSION_URL, fetcher);
}

export function useBrokers() {
  return useSWR<string[]>(BROKERS_URL, fetcher);
}

export function useBrokerInfo() {
  return useSWR<BrokerInfo[]>(BROKER_INFO_URL, fetcher);
}

export function useUploadHistory() {
  return useSWR<UploadLogItem[]>(UPLOAD_HISTORY_URL, fetcher);
}

// ---------------------------------------------------------------------------
// Global revalidation after upload
// ---------------------------------------------------------------------------

export function revalidateAll() {
  void globalMutate(() => true);
}
