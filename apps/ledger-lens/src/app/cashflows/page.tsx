"use client";

import { useYear } from "@/context/YearContext";
import { useCashflows, useDepositTimeseries } from "@/hooks/useStatement";
import { DepositsTable } from "@/components/cashflows/DepositsTable";
import { CashFlowBarChart } from "@/components/cashflows/CashFlowBarChart";
import { DepositVsGrowthChart } from "@/components/trends/DepositVsGrowthChart";
import { KpiCard } from "@/components/overview/KpiCard";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { fmtUsd } from "@/lib/formatters";

export default function CashFlowsPage() {
  const { selectedYear } = useYear();
  const { data, isLoading, error } = useCashflows(selectedYear);
  const {
    data: depositTimeseries,
    isLoading: depositTsLoading,
    error: depositTsError,
  } = useDepositTimeseries();

  const currencies = data ? Object.entries(data.by_currency) : [];

  return (
    <div className="space-y-6">
      {/* ── All-time deposits summary ────────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-widest text-muted-foreground">
          All Years
        </h2>
        {depositTsError ? (
          <ApiErrorState error={depositTsError} compact />
        ) : depositTsLoading || !depositTimeseries ? (
          <Skeleton className="h-56 w-full rounded-lg" />
        ) : depositTimeseries.length > 0 ? (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <KpiCard
                title="Total Deposited (All Years)"
                value={fmtUsd(
                  depositTimeseries.at(-1)?.cumulative_deposits ?? 0,
                )}
                subtitle={`Across ${depositTimeseries.length} year${depositTimeseries.length !== 1 ? "s" : ""}`}
              />
              <KpiCard
                title="Average Annual Deposit"
                value={fmtUsd(
                  (depositTimeseries.at(-1)?.cumulative_deposits ?? 0) /
                    depositTimeseries.length,
                )}
                subtitle="Per year"
              />
            </div>
            <DepositVsGrowthChart data={depositTimeseries} />
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No data yet.</p>
        )}
      </div>

      <Separator />

      {/* ── Selected-year detail ─────────────────────────────────────────── */}
      {selectedYear ? (
        <div className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-widest text-muted-foreground">
            {selectedYear} Detail
          </h2>
          {error ? (
            <ApiErrorState error={error} compact />
          ) : isLoading || !data ? (
            <Skeleton className="h-64 w-full" />
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <KpiCard
                  title="Total Deposits (USD equiv.)"
                  value={fmtUsd(data.total_usd)}
                />
                {currencies.map(([ccy, amount]) => (
                  <KpiCard
                    key={ccy}
                    title={`${ccy} Deposits`}
                    value={amount.toLocaleString("en-US", {
                      maximumFractionDigits: 2,
                    })}
                    subtitle={ccy}
                  />
                ))}
              </div>
              <CashFlowBarChart data={data.monthly} />
              <div>
                <h3 className="mb-3 text-base font-medium">Deposit History</h3>
                <DepositsTable deposits={data.deposits} />
              </div>
            </>
          )}
        </div>
      ) : (
        <p className="text-muted-foreground">
          Select a year to view cash flow detail.
        </p>
      )}
    </div>
  );
}
