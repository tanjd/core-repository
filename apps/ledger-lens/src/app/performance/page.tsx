"use client";

import { useYear } from "@/context/YearContext";
import {
  useCommissionTimeseries,
  useNavTimeseries,
  usePerformance,
} from "@/hooks/useStatement";
import { PnlTable } from "@/components/performance/PnlTable";
import { MtmTable } from "@/components/performance/MtmTable";
import { KpiCard } from "@/components/overview/KpiCard";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { fmtUsd, fmtPct, pnlColor } from "@/lib/formatters";

export default function PerformancePage() {
  const { selectedYear } = useYear();
  const { data: perfData, isLoading } = usePerformance(selectedYear);
  const { data: commTimeseries } = useCommissionTimeseries();
  const { data: navTimeseries } = useNavTimeseries();

  const totalCommissions = Math.abs(
    (commTimeseries ?? []).reduce((s, c) => s + c.total, 0),
  );
  const latestNav = (navTimeseries ?? []).at(-1)?.nav_current ?? 1;
  const dragPct = latestNav > 0 ? (totalCommissions / latestNav) * 100 : 0;

  if (!selectedYear) {
    return <p className="text-muted-foreground">Select a year to view P&L.</p>;
  }

  if (isLoading || !perfData) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">P&L Analysis — {selectedYear}</h1>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          title="Short-Term Realized"
          value={fmtUsd(perfData?.summary.realized_st ?? 0)}
          valueClass={pnlColor(perfData?.summary.realized_st ?? 0)}
        />
        <KpiCard
          title="Long-Term Realized"
          value={fmtUsd(perfData?.summary.realized_lt ?? 0)}
          valueClass={pnlColor(perfData?.summary.realized_lt ?? 0)}
        />
        <KpiCard
          title="Total Commissions Paid"
          value={fmtUsd(totalCommissions)}
          valueClass="text-red-500 dark:text-red-400"
          subtitle="All years combined"
        />
        <KpiCard
          title="Commission Drag"
          value={fmtPct(dragPct)}
          valueClass="text-red-500 dark:text-red-400"
          subtitle="of current portfolio"
        />
      </div>
      <Tabs defaultValue="pnl">
        <TabsList>
          <TabsTrigger value="pnl">P&L by Symbol</TabsTrigger>
          <TabsTrigger value="mtm">Mark-to-Market</TabsTrigger>
          <TabsTrigger value="corporate">
            Corporate Actions ({perfData.corporate_actions.length})
          </TabsTrigger>
        </TabsList>
        <TabsContent value="pnl" className="mt-4">
          <PnlTable records={perfData.pnl_records} summary={perfData.summary} />
        </TabsContent>
        <TabsContent value="mtm" className="mt-4">
          <MtmTable items={perfData.mtm_summary} />
        </TabsContent>
        <TabsContent value="corporate" className="mt-4">
          {perfData.corporate_actions.length === 0 ? (
            <p className="py-4 text-sm text-muted-foreground">
              No corporate actions.
            </p>
          ) : (
            <div className="rounded-md border text-sm">
              {perfData.corporate_actions.map((ca, i) => (
                <div
                  key={i}
                  className="flex items-center justify-between border-b px-4 py-2 last:border-0"
                >
                  <div>
                    <span className="font-medium">{ca.symbol}</span>
                    <span className="ml-2 text-muted-foreground">
                      {ca.description}
                    </span>
                  </div>
                  <span className="font-mono">{ca.action_date}</span>
                </div>
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
