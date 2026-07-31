"use client";

import { useYear } from "@/context/YearContext";
import {
  useIncome,
  useHoldings,
  useDividendTimeseries,
} from "@/hooks/useStatement";
import { IncomeSummaryCards } from "@/components/income/IncomeSummaryCards";
import { DividendTable } from "@/components/income/DividendTable";
import { WithholdingTaxTable } from "@/components/income/WithholdingTaxTable";
import { DividendGrowthChart } from "@/components/trends/DividendGrowthChart";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KpiCard } from "@/components/overview/KpiCard";
import { fmtUsd, fmtPct, pnlColor } from "@/lib/formatters";

function buildYieldRows(
  dividends: { symbol: string; gross_amount: number }[],
  costBasisMap: Map<string, number>,
) {
  const grossBySymbol = dividends.reduce<Record<string, number>>((acc, d) => {
    acc[d.symbol] = (acc[d.symbol] ?? 0) + d.gross_amount;
    return acc;
  }, {});

  return Object.entries(grossBySymbol)
    .map(([symbol, gross]) => {
      const costBasis = costBasisMap.get(symbol);
      return {
        symbol,
        gross,
        yieldOnCost: costBasis ? (gross / costBasis) * 100 : null,
      };
    })
    .sort((a, b) => b.gross - a.gross);
}

export default function IncomePage() {
  const { selectedYear } = useYear();
  const { data, isLoading } = useIncome(selectedYear);
  const { data: holdingsData } = useHoldings(selectedYear);
  const { data: dividendTimeseries, isLoading: dividendTsLoading } =
    useDividendTimeseries();

  const costBasisMap = new Map(
    (holdingsData?.positions ?? []).map((p) => [p.symbol, p.cost_basis]),
  );
  const yieldRows = data ? buildYieldRows(data.dividends, costBasisMap) : [];

  // Forward dividend estimate: sum per-share dividend rates per symbol this year,
  // then multiply by current holdings quantity to project next year's income.
  const dpsBySymbol = (data?.dividends ?? []).reduce<Record<string, number>>(
    (acc, d) => ({
      ...acc,
      [d.symbol]: (acc[d.symbol] ?? 0) + d.per_share_rate,
    }),
    {},
  );
  const forwardEstimate = (holdingsData?.positions ?? []).reduce(
    (sum, h) => sum + (dpsBySymbol[h.symbol] ?? 0) * h.quantity,
    0,
  );

  return (
    <div className="space-y-6">
      {/* ── All-time income summary ───────────────────────────────────────── */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-widest text-muted-foreground">
          All Years
        </h2>
        {dividendTsLoading || !dividendTimeseries ? (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-24 w-full rounded-lg" />
              ))}
            </div>
            <Skeleton className="h-56 w-full rounded-lg" />
          </div>
        ) : dividendTimeseries.length > 0 ? (
          <div className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <KpiCard
                title="Total Gross Dividends"
                value={fmtUsd(
                  dividendTimeseries.reduce((s, d) => s + d.gross, 0),
                )}
                subtitle="All years combined"
                valueClass={pnlColor(1)}
              />
              <KpiCard
                title="Total Withholding Tax"
                value={fmtUsd(
                  dividendTimeseries.reduce((s, d) => s + d.withholding, 0),
                )}
                subtitle="Deducted at source"
                valueClass={pnlColor(-1)}
              />
              <KpiCard
                title="Total Net Dividends"
                value={fmtUsd(
                  dividendTimeseries.reduce((s, d) => s + d.net, 0),
                )}
                subtitle="After withholding"
                valueClass={pnlColor(1)}
              />
              <KpiCard
                title="Total Fees Paid"
                value={fmtUsd(
                  Math.abs(dividendTimeseries.reduce((s, d) => s + d.fees, 0)),
                )}
                subtitle="Excl. commission adjustments"
                valueClass={pnlColor(-1)}
              />
            </div>
            <DividendGrowthChart data={dividendTimeseries} />
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
          {isLoading || !data ? (
            <div className="space-y-4">
              <Skeleton className="h-24 w-full" />
              <Skeleton className="h-64 w-full" />
            </div>
          ) : (
            <>
              <IncomeSummaryCards summary={data.summary} />
              <div className="mt-4">
                <KpiCard
                  title="Forward Dividend Estimate"
                  value={fmtUsd(forwardEstimate)}
                  subtitle={`Based on ${selectedYear} per-share rates × current holdings`}
                  valueClass={pnlColor(forwardEstimate > 0 ? 1 : 0)}
                />
              </div>
              <Tabs defaultValue="dividends">
                <TabsList>
                  <TabsTrigger value="dividends">
                    Dividends ({data.dividends.length})
                  </TabsTrigger>
                  <TabsTrigger value="withholding">
                    Withholding Tax ({data.withholding_tax.length})
                  </TabsTrigger>
                  <TabsTrigger value="fees">
                    Fees ({data.fees.length})
                  </TabsTrigger>
                </TabsList>
                <TabsContent value="dividends" className="mt-4 space-y-4">
                  {yieldRows.length > 0 && (
                    <Card>
                      <CardHeader className="pb-1">
                        <CardTitle className="text-sm font-medium text-muted-foreground">
                          Dividends by Symbol
                        </CardTitle>
                      </CardHeader>
                      <CardContent className="p-0">
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>Symbol</TableHead>
                              <TableHead className="text-right">
                                Total Gross
                              </TableHead>
                              <TableHead className="text-right">
                                Yield on Cost
                              </TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {yieldRows.map((r) => (
                              <TableRow key={r.symbol}>
                                <TableCell className="font-medium">
                                  {r.symbol}
                                </TableCell>
                                <TableCell className="text-right font-mono text-green-600">
                                  {fmtUsd(r.gross)}
                                </TableCell>
                                <TableCell className="text-right font-mono">
                                  {r.yieldOnCost != null
                                    ? fmtPct(r.yieldOnCost)
                                    : "—"}
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </CardContent>
                    </Card>
                  )}
                  <DividendTable dividends={data.dividends} />
                </TabsContent>
                <TabsContent value="withholding" className="mt-4">
                  <WithholdingTaxTable items={data.withholding_tax} />
                </TabsContent>
                <TabsContent value="fees" className="mt-4">
                  {data.fees.length === 0 ? (
                    <p className="py-4 text-sm text-muted-foreground">
                      No fees recorded.
                    </p>
                  ) : (
                    <div className="rounded-md border text-sm">
                      {data.fees.map((f, i) => (
                        <div
                          key={i}
                          className="flex justify-between border-b px-4 py-2 last:border-0"
                        >
                          <span className="text-muted-foreground">
                            {f.description}
                          </span>
                          <span className="font-mono">
                            {f.amount >= 0 ? "+" : ""}
                            {f.amount.toFixed(2)}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </>
          )}
        </div>
      ) : (
        <p className="text-muted-foreground">
          Select a year to view income detail.
        </p>
      )}
    </div>
  );
}
