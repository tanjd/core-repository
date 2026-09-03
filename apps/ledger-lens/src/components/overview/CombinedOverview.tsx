"use client";

import { AllTimeSummaryCards } from "@/components/overview/AllTimeSummaryCards";
import { YearSummaryTable } from "@/components/overview/YearSummaryTable";
import { NavVsInvestedChart } from "@/components/overview/NavVsInvestedChart";
import { PortfolioAllocationChart } from "@/components/holdings/PortfolioAllocationChart";
import { KpiCard } from "@/components/overview/KpiCard";
import { AssetAllocationChart } from "@/components/overview/AssetAllocationChart";
import { ChangeInNavTable } from "@/components/overview/ChangeInNavTable";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { fmtUsd, fmtPct, pnlColor } from "@/lib/formatters";
import type {
  DcaItem,
  DepositTimeseriesItem,
  DividendTimeseriesItem,
  HoldingsResponse,
  NavTimeseriesItem,
  OverviewResponse,
  PnlTimeseriesItem,
  XirrTimeseriesItem,
} from "@/lib/types";

interface Props {
  selectedYear: number | null;
  setSelectedYear: (year: number) => void;
  selectedBroker: string | null;
  hasMoomoo: boolean;
  timeseriesLoading: boolean;
  navData: NavTimeseriesItem[] | undefined;
  depositData: DepositTimeseriesItem[] | undefined;
  dividendData: DividendTimeseriesItem[] | undefined;
  pnlData: PnlTimeseriesItem[] | undefined;
  dcaData: DcaItem[] | undefined;
  xirrData: XirrTimeseriesItem[] | undefined;
  latestHoldings: HoldingsResponse | undefined;
  latestYear: number | null;
  yearData: OverviewResponse | undefined;
  yearLoading: boolean;
  yearError: Error | undefined;
}

export function CombinedOverview({
  selectedYear,
  setSelectedYear,
  selectedBroker,
  hasMoomoo,
  timeseriesLoading,
  navData,
  depositData,
  dividendData,
  pnlData,
  dcaData,
  xirrData,
  latestHoldings,
  latestYear,
  yearData,
  yearLoading,
  yearError,
}: Props) {
  const yearRange =
    navData && navData.length > 1
      ? `${navData[0].year}–${navData.at(-1)!.year}`
      : navData?.length === 1
        ? String(navData[0].year)
        : null;

  return (
    <div className="space-y-6">
      {selectedBroker === "ibkr" && (
        <div>
          <h1 className="text-xl font-semibold text-blue-600 dark:text-blue-400">
            IBKR Overview
            {selectedYear && (
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                — {selectedYear}
              </span>
            )}
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Change year using selector above
          </p>
        </div>
      )}

      {/* ── All-time summary ─────────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-widest text-muted-foreground">
          All Time{yearRange ? ` · ${yearRange}` : ""}
        </h2>
        {timeseriesLoading ||
        !navData ||
        !depositData ||
        !dividendData ||
        !pnlData ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-24 w-full rounded-lg" />
            ))}
          </div>
        ) : (
          <AllTimeSummaryCards
            navData={navData}
            depositData={depositData}
            dividendData={dividendData}
            pnlData={pnlData}
            dcaData={dcaData}
            xirrData={xirrData}
            hasMoomoo={hasMoomoo && !selectedBroker}
          />
        )}
      </div>

      {/* Year-by-year table */}
      {!timeseriesLoading && navData && navData.length > 0 ? (
        <YearSummaryTable
          navData={navData}
          depositData={depositData ?? []}
          dividendData={dividendData ?? []}
          pnlData={pnlData ?? []}
          selectedYear={selectedYear}
          onSelectYear={setSelectedYear}
        />
      ) : (
        <Skeleton className="h-32 w-full rounded-lg" />
      )}

      {/* NAV vs Total Invested */}
      {!timeseriesLoading && navData && depositData && navData.length > 0 && (
        <NavVsInvestedChart
          navData={navData}
          depositData={depositData}
          hasMoomoo={hasMoomoo}
        />
      )}

      {/* Current portfolio snapshot */}
      {latestHoldings &&
        latestHoldings.positions.length > 0 &&
        (() => {
          const positions =
            selectedBroker === "ibkr"
              ? latestHoldings.positions.filter((p) => p.broker === "ibkr")
              : latestHoldings.positions;
          return positions.length > 0 ? (
            <div>
              <h2 className="mb-3 text-sm font-semibold uppercase tracking-widest text-muted-foreground">
                Current Portfolio ({latestYear})
              </h2>
              <PortfolioAllocationChart
                positions={positions}
                title="Holdings by Market Value"
              />
            </div>
          ) : null;
        })()}

      {/* Selected-year detail */}
      {selectedYear && (
        <div className="rounded-xl border bg-muted/30 p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="h-5 w-1 rounded-full bg-primary" />
              <h2 className="text-base font-semibold">
                {selectedYear} Snapshot
              </h2>
            </div>
            <span className="text-xs text-muted-foreground">
              Change year using the selector above
            </span>
          </div>

          {yearError ? (
            <ApiErrorState error={yearError} compact />
          ) : yearLoading || !yearData ? (
            <>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-24 w-full rounded-lg" />
                ))}
              </div>
              <Skeleton className="h-48 w-full rounded-lg" />
            </>
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <span>
                  {yearData.period} · {yearData.account_name} ·{" "}
                  {yearData.account_id}
                </span>
                {yearData.broker_breakdown.length > 1 && (
                  <span className="flex gap-2">
                    {yearData.broker_breakdown.map((b) => (
                      <span
                        key={b.broker}
                        className="rounded border px-1.5 py-0.5 font-medium"
                      >
                        {b.broker === "ibkr" ? "IBKR" : "Moomoo"}:{" "}
                        {fmtUsd(b.nav_current)}
                      </span>
                    ))}
                  </span>
                )}
              </div>

              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
                <KpiCard
                  title="Year-End Portfolio Value"
                  value={fmtUsd(yearData.nav.current_total)}
                  subtitle={`Prior: ${fmtUsd(yearData.nav.prior_total)}`}
                />
                <KpiCard
                  title="Time-Weighted Return"
                  value={fmtPct(yearData.nav.twr_pct)}
                  valueClass={pnlColor(yearData.nav.twr_pct)}
                />
                <KpiCard
                  title="NAV Change"
                  value={fmtUsd(
                    yearData.nav.current_total - yearData.nav.prior_total,
                  )}
                  valueClass={pnlColor(
                    yearData.nav.current_total - yearData.nav.prior_total,
                  )}
                  subtitle={`${(((yearData.nav.current_total - yearData.nav.prior_total) / (yearData.nav.prior_total || 1)) * 100).toFixed(2)}%`}
                />
                <KpiCard
                  title="Net Deposits"
                  value={fmtUsd(yearData.change_in_nav.deposits_withdrawals)}
                  valueClass={pnlColor(
                    yearData.change_in_nav.deposits_withdrawals,
                  )}
                />
                <KpiCard
                  title="Unrealized P&L"
                  value={fmtUsd(
                    pnlData?.find((d) => d.year === selectedYear)?.unrealized ??
                      0,
                  )}
                  valueClass={pnlColor(
                    pnlData?.find((d) => d.year === selectedYear)?.unrealized ??
                      0,
                  )}
                  subtitle="Open positions"
                />
              </div>

              <div className="grid gap-6 lg:grid-cols-2">
                <AssetAllocationChart data={yearData.asset_allocation} />
                <ChangeInNavTable data={yearData.change_in_nav} />
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
