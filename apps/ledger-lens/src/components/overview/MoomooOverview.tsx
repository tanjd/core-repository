"use client";

import { useHoldings, useStockTrades } from "@/hooks/useStatement";
import { PortfolioAllocationChart } from "@/components/holdings/PortfolioAllocationChart";
import { UnrealizedPnlChart } from "@/components/holdings/UnrealizedPnlChart";
import { KpiCard } from "@/components/overview/KpiCard";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { fmtUsd, fmtSgd, pnlColor } from "@/lib/formatters";

interface Props {
  year: number | null;
  impliedRate?: number | null;
}

export function MoomooOverview({ year, impliedRate }: Props) {
  const {
    data: holdings,
    isLoading: holdingsLoading,
    error: holdingsError,
  } = useHoldings(year);
  const {
    data: trades,
    isLoading: tradesLoading,
    error: tradesError,
  } = useStockTrades(year);

  const loadError = holdingsError ?? tradesError;
  if (loadError) {
    return <ApiErrorState error={loadError} />;
  }

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
