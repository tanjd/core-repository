"use client";

import { useYear } from "@/context/YearContext";
import { useBroker } from "@/context/BrokerContext";
import {
  useOverview,
  useHoldings,
  useNavTimeseries,
  useDepositTimeseries,
  useDividendTimeseries,
  usePnlTimeseries,
  useDcaTimeseries,
  useStockTrades,
} from "@/hooks/useStatement";
import { AllTimeSummaryCards } from "@/components/overview/AllTimeSummaryCards";
import { YearSummaryTable } from "@/components/overview/YearSummaryTable";
import { NavVsInvestedChart } from "@/components/overview/NavVsInvestedChart";
import { PortfolioAllocationChart } from "@/components/holdings/PortfolioAllocationChart";
import { UnrealizedPnlChart } from "@/components/holdings/UnrealizedPnlChart";
import { KpiCard } from "@/components/overview/KpiCard";
import { AssetAllocationChart } from "@/components/overview/AssetAllocationChart";
import { ChangeInNavTable } from "@/components/overview/ChangeInNavTable";
import { Skeleton } from "@/components/ui/skeleton";
import { fmtUsd, fmtSgd, fmtPct, pnlColor } from "@/lib/formatters";
import { UploadDialog } from "@/components/layout/UploadDialog";

// ── Moomoo-specific overview ─────────────────────────────────────────────────
function MoomooOverview({
  year,
  impliedRate,
}: {
  year: number | null;
  impliedRate?: number | null;
}) {
  const { data: holdings, isLoading: holdingsLoading } = useHoldings(year);
  const { data: trades, isLoading: tradesLoading } = useStockTrades(year);

  const moomooPositions = (holdings?.positions ?? []).filter(
    (p) => p.broker === "moomoo",
  );
  const moomooTrades = (trades?.trades ?? []).filter(
    (t) => t.broker === "moomoo",
  );

  const portfolioValue = moomooPositions.reduce(
    (s, p) => s + p.current_value,
    0,
  );
  const unrealizedPnl = moomooPositions.reduce(
    (s, p) => s + p.unrealized_pnl,
    0,
  );

  if (holdingsLoading || tradesLoading) {
    return (
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 w-full rounded-lg" />
          ))}
        </div>
        <Skeleton className="h-48 w-full rounded-lg" />
      </div>
    );
  }

  if (moomooPositions.length === 0) {
    return (
      <p className="text-muted-foreground">
        No Moomoo positions for {year ?? "this year"}.
      </p>
    );
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          title="Portfolio Value"
          value={fmtUsd(portfolioValue)}
          subtitle={
            impliedRate
              ? `≈ ${fmtSgd(portfolioValue * impliedRate)} SGD · Market value`
              : "Market value"
          }
        />
        <KpiCard
          title="Unrealized P&L"
          value={fmtUsd(unrealizedPnl)}
          valueClass={pnlColor(unrealizedPnl)}
          subtitle="Open positions"
        />
        <KpiCard
          title="Open Positions"
          value={String(moomooPositions.length)}
          subtitle={`${year ?? ""}`}
        />
        <KpiCard
          title="Trades This Year"
          value={String(moomooTrades.length)}
          subtitle={`${year ?? ""}`}
        />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <PortfolioAllocationChart
          positions={moomooPositions}
          title="Moomoo Allocation"
        />
        <UnrealizedPnlChart positions={moomooPositions} />
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export default function OverviewPage() {
  const { selectedYear, setSelectedYear } = useYear();
  const { brokerList, selectedBroker } = useBroker();
  const hasMoomoo = brokerList.includes("moomoo");

  // When IBKR is explicitly selected, filter timeseries to IBKR-only so numbers exclude Moomoo
  const timeseriesBroker = selectedBroker === "ibkr" ? "ibkr" : undefined;

  // All-time data (timeseries)
  const { data: navData, isLoading: navLoading } =
    useNavTimeseries(timeseriesBroker);
  const { data: depositData, isLoading: depositLoading } =
    useDepositTimeseries(timeseriesBroker);
  const { data: dividendData, isLoading: dividendLoading } =
    useDividendTimeseries(timeseriesBroker);
  const { data: pnlData, isLoading: pnlLoading } =
    usePnlTimeseries(timeseriesBroker);
  const { data: dcaData } = useDcaTimeseries();

  // Latest year's holdings for the current portfolio snapshot (combined overview only)
  const latestYear = navData?.at(-1)?.year ?? null;
  const { data: latestHoldings } = useHoldings(latestYear);
  // Selected-year detail — use broker filter when IBKR is selected so NAV reflects IBKR-only
  const { data: yearData, isLoading: yearLoading } = useOverview(
    selectedYear,
    selectedBroker === "ibkr" ? "ibkr" : undefined,
  );

  const timeseriesLoading =
    navLoading || depositLoading || dividendLoading || pnlLoading;
  const hasData = (navData?.length ?? 0) > 0;

  const totalDepositsValue = depositData?.at(-1)?.cumulative_deposits ?? 0;
  const totalSgdDeposits = (dcaData ?? []).reduce((s, d) => s + d.sgd, 0);
  const impliedRate =
    totalDepositsValue > 0 && totalSgdDeposits > 0
      ? totalSgdDeposits / totalDepositsValue
      : null;

  // Empty state
  if (!hasData && !timeseriesLoading) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 text-center">
        <p className="text-muted-foreground">No data imported yet.</p>
        <UploadDialog />
      </div>
    );
  }

  // ── Moomoo-specific overview ──────────────────────────────────────────────
  if (selectedBroker === "moomoo") {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-xl font-semibold text-orange-600 dark:text-orange-400">
            Moomoo Overview
            {selectedYear && (
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                — {selectedYear}
              </span>
            )}
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Holdings-based snapshot · change year using selector above
          </p>
        </div>
        <MoomooOverview year={selectedYear} impliedRate={impliedRate} />
      </div>
    );
  }

  // ── Combined / IBKR overview ──────────────────────────────────────────────
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

          {yearLoading || !yearData ? (
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
