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
  useXirrTimeseries,
} from "@/hooks/useStatement";
import { MoomooOverview } from "@/components/overview/MoomooOverview";
import { CombinedOverview } from "@/components/overview/CombinedOverview";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { UploadDialog } from "@/components/layout/UploadDialog";

export default function OverviewPage() {
  const { selectedYear, setSelectedYear } = useYear();
  const { brokerList, selectedBroker } = useBroker();
  const hasMoomoo = brokerList.includes("moomoo");

  // When IBKR is explicitly selected, filter timeseries to IBKR-only so numbers exclude Moomoo
  const timeseriesBroker = selectedBroker === "ibkr" ? "ibkr" : undefined;

  // All-time data (timeseries)
  const {
    data: navData,
    isLoading: navLoading,
    error: navError,
  } = useNavTimeseries(timeseriesBroker);
  const {
    data: depositData,
    isLoading: depositLoading,
    error: depositError,
  } = useDepositTimeseries(timeseriesBroker);
  const {
    data: dividendData,
    isLoading: dividendLoading,
    error: dividendError,
  } = useDividendTimeseries(timeseriesBroker);
  const {
    data: pnlData,
    isLoading: pnlLoading,
    error: pnlError,
  } = usePnlTimeseries(timeseriesBroker);
  const { data: dcaData } = useDcaTimeseries();
  // XIRR v1 is IBKR-only (see docs/benchmark-and-money-weighted-returns-spec.md) — always ask
  // for the IBKR series regardless of the active broker filter; the card hides itself when
  // there's no IBKR data to compute from.
  const { data: xirrData } = useXirrTimeseries("ibkr");

  // Latest year's holdings for the current portfolio snapshot (combined overview only)
  const latestYear = navData?.at(-1)?.year ?? null;
  const { data: latestHoldings } = useHoldings(latestYear);
  // Selected-year detail — use broker filter when IBKR is selected so NAV reflects IBKR-only
  const {
    data: yearData,
    isLoading: yearLoading,
    error: yearError,
  } = useOverview(selectedYear, selectedBroker === "ibkr" ? "ibkr" : undefined);

  const timeseriesLoading =
    navLoading || depositLoading || dividendLoading || pnlLoading;
  const hasData = (navData?.length ?? 0) > 0;
  const timeseriesError = navError ?? depositError ?? dividendError ?? pnlError;

  // Error state — a real fetch failure, not "no data imported yet"
  if (timeseriesError) {
    return <ApiErrorState error={timeseriesError} />;
  }

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
  return (
    <CombinedOverview
      selectedYear={selectedYear}
      setSelectedYear={setSelectedYear}
      selectedBroker={selectedBroker}
      hasMoomoo={hasMoomoo}
      timeseriesLoading={timeseriesLoading}
      navData={navData}
      depositData={depositData}
      dividendData={dividendData}
      pnlData={pnlData}
      dcaData={dcaData}
      xirrData={xirrData}
      latestHoldings={latestHoldings}
      latestYear={latestYear}
      yearData={yearData}
      yearLoading={yearLoading}
      yearError={yearError}
    />
  );
}
