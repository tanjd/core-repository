"use client";

import { useYear } from "@/context/YearContext";
import { useBroker } from "@/context/BrokerContext";
import {
  useStockTrades,
  useForexTrades,
  useCommissionTimeseries,
} from "@/hooks/useStatement";
import { TradesTable } from "@/components/trades/TradesTable";
import { CommissionChart } from "@/components/trades/CommissionChart";
import { KpiCard } from "@/components/overview/KpiCard";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { fmtUsd, pnlColor } from "@/lib/formatters";

export default function TradesPage() {
  const { selectedYear } = useYear();
  const { selectedBroker } = useBroker();
  const {
    data: stocks,
    isLoading: loadingStocks,
    error: stocksError,
  } = useStockTrades(selectedYear);
  const {
    data: forex,
    isLoading: loadingForex,
    error: forexError,
  } = useForexTrades(selectedYear);
  const {
    data: commTimeseries,
    isLoading: commLoading,
    error: commError,
  } = useCommissionTimeseries();

  if (!selectedYear) {
    return (
      <p className="text-muted-foreground">Select a year to view trades.</p>
    );
  }

  const loadError = stocksError ?? forexError ?? commError;
  if (loadError) {
    return <ApiErrorState error={loadError} />;
  }

  // Apply broker filter client-side (trades already have broker field)
  const visibleStocks = selectedBroker
    ? (stocks?.trades ?? []).filter((t) => t.broker === selectedBroker)
    : (stocks?.trades ?? []);
  const visibleForex = selectedBroker
    ? (forex?.trades ?? []).filter((t) => t.broker === selectedBroker)
    : (forex?.trades ?? []);

  // Per-year commission totals computed from visible (filtered) trades
  const stockComm = visibleStocks.reduce((s, t) => s + t.commission, 0);
  const forexComm = visibleForex.reduce((s, t) => s + t.commission, 0);
  const totalComm = stockComm + forexComm;

  const allYearsTotalComm = commTimeseries
    ? Math.abs(commTimeseries.reduce((s, d) => s + d.total, 0))
    : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Trades</h1>

      {/* All Years */}
      <div className="space-y-4">
        <h2 className="text-sm font-semibold uppercase tracking-widest text-muted-foreground">
          All Years
        </h2>
        {commLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : commTimeseries && commTimeseries.length > 0 ? (
          <>
            <div className="grid gap-4 sm:grid-cols-2">
              <KpiCard
                title="Total Commissions (All Years)"
                value={fmtUsd(allYearsTotalComm)}
                subtitle={`Across ${commTimeseries.length} year${commTimeseries.length !== 1 ? "s" : ""}`}
                valueClass={pnlColor(-1)}
              />
              <KpiCard
                title="Average Annual Commission"
                value={fmtUsd(allYearsTotalComm / commTimeseries.length)}
                subtitle="Per year"
                valueClass={pnlColor(-1)}
              />
            </div>
            <CommissionChart data={commTimeseries} />
          </>
        ) : (
          <p className="text-sm text-muted-foreground">
            No commission data available.
          </p>
        )}
      </div>

      <Separator />

      {/* Per-Year Detail */}
      <div className="space-y-4">
        <h2 className="text-sm font-semibold uppercase tracking-widest text-muted-foreground">
          {selectedYear} Detail
        </h2>

        {loadingStocks || loadingForex ? (
          <Skeleton className="h-24 w-full" />
        ) : stocks || forex ? (
          <div className="grid gap-4 sm:grid-cols-3">
            <KpiCard
              title="Total Commissions"
              value={fmtUsd(Math.abs(totalComm))}
              subtitle={`${visibleStocks.length + visibleForex.length} trades`}
              valueClass={pnlColor(-1)}
            />
            <KpiCard
              title="Stock Commissions"
              value={fmtUsd(Math.abs(stockComm))}
              subtitle={`${visibleStocks.length} stock trades`}
              valueClass={pnlColor(-1)}
            />
            <KpiCard
              title="Forex Commissions"
              value={fmtUsd(Math.abs(forexComm))}
              subtitle={`${visibleForex.length} forex trades`}
              valueClass={pnlColor(-1)}
            />
          </div>
        ) : null}

        <Tabs defaultValue="stocks">
          <TabsList>
            <TabsTrigger value="stocks">
              Stocks {stocks ? `(${visibleStocks.length})` : ""}
            </TabsTrigger>
            <TabsTrigger value="forex">
              Forex {forex ? `(${visibleForex.length})` : ""}
            </TabsTrigger>
          </TabsList>
          <TabsContent value="stocks" className="mt-4">
            {loadingStocks || !stocks ? (
              <Skeleton className="h-48 w-full" />
            ) : (
              <TradesTable trades={visibleStocks} />
            )}
          </TabsContent>
          <TabsContent value="forex" className="mt-4">
            {loadingForex || !forex ? (
              <Skeleton className="h-48 w-full" />
            ) : (
              <TradesTable trades={visibleForex} currencyLabel="Currency" />
            )}
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
