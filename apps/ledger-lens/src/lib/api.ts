import type {
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
  PreviewResponse,
  TradesResponse,
  UploadLogItem,
  UploadResponse,
  VersionResponse,
} from "@/lib/types";

// Empty string = relative URL → browser calls /api/... on the same host.
// Next.js rewrites (next.config.ts) proxy those requests to the FastAPI backend.
// Set NEXT_PUBLIC_API_URL only if you need to hit a backend on a different origin
// (e.g. local dev without Docker, pointing at a remote staging server).
const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

export async function fetcher<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`);
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    throw new Error(`API error ${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

// ---------------------------------------------------------------------------
// Per-year endpoints
// ---------------------------------------------------------------------------

export function getOverviewUrl(year: number, broker?: string) {
  return `/api/overview?year=${year}${broker ? `&broker=${broker}` : ""}`;
}

export function getHoldingsUrl(year: number) {
  return `/api/holdings?year=${year}`;
}

export function getTradesUrl(year: number, type: "stock" | "forex") {
  return `/api/trades?year=${year}&type=${type}`;
}

export function getIncomeUrl(year: number) {
  return `/api/income?year=${year}`;
}

export function getCashflowsUrl(year: number) {
  return `/api/cashflows?year=${year}`;
}

export function getPerformanceUrl(year: number) {
  return `/api/performance?year=${year}`;
}

// ---------------------------------------------------------------------------
// Timeseries endpoints
// ---------------------------------------------------------------------------

export const VERSION_URL = "/api/version";
export const BROKERS_URL = "/api/brokers";
export const BROKER_INFO_URL = "/api/broker-info";
export const UPLOAD_HISTORY_URL = "/api/upload-history";

export function getTimeseriesUrl(
  key: "nav" | "deposits" | "dividends" | "pnl",
  broker?: string,
) {
  const base = `/api/timeseries/${key}`;
  return broker ? `${base}?broker=${broker}` : base;
}

export const TIMESERIES_URLS = {
  nav: "/api/timeseries/nav",
  deposits: "/api/timeseries/deposits",
  dividends: "/api/timeseries/dividends",
  pnl: "/api/timeseries/pnl",
  dca: "/api/timeseries/dca",
  positions: "/api/timeseries/positions",
  commissions: "/api/timeseries/commissions",
} as const;

// ---------------------------------------------------------------------------
// Upload / Preview (imperative — not SWR)
// ---------------------------------------------------------------------------

export async function previewStatement(file: File): Promise<PreviewResponse> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${BASE_URL}/api/preview`, {
    method: "POST",
    body: form,
  });
  if (!res.ok) {
    const data = (await res.json().catch(() => ({}))) as { detail?: string };
    throw new Error(data.detail ?? `Preview failed (${res.status})`);
  }
  return res.json() as Promise<PreviewResponse>;
}

export async function uploadStatement(file: File): Promise<UploadResponse> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${BASE_URL}/api/upload`, {
    method: "POST",
    body: form,
  });
  if (!res.ok) {
    const data = (await res.json().catch(() => ({}))) as { detail?: string };
    throw new Error(data.detail ?? `Upload failed (${res.status})`);
  }
  return res.json() as Promise<UploadResponse>;
}

export async function fetchYears(): Promise<number[]> {
  return fetcher<number[]>("/api/years");
}

// Re-export typed fetchers for SWR
export {
  type CashflowsResponse,
  type CommissionTimeseriesItem,
  type DcaItem,
  type DepositTimeseriesItem,
  type DividendTimeseriesItem,
  type HoldingsResponse,
  type IncomeResponse,
  type NavTimeseriesItem,
  type OverviewResponse,
  type PerformanceResponse,
  type PnlTimeseriesItem,
  type PositionTimeseriesItem,
  type TradesResponse,
  type UploadLogItem,
  type VersionResponse,
};
