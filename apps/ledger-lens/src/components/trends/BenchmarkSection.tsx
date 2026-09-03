"use client";

import { useState } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiErrorState } from "@/components/layout/ApiErrorState";
import { BenchmarkUploadDialog } from "@/components/trends/BenchmarkUploadDialog";
import { BenchmarkComparisonChart } from "@/components/trends/BenchmarkComparisonChart";
import { TwrVsBenchmarkChart } from "@/components/trends/TwrVsBenchmarkChart";
import { useBenchmarks, useBenchmarkTimeseries } from "@/hooks/useStatement";

export function BenchmarkSection() {
  const { data: benchmarks, isLoading: benchmarksLoading } = useBenchmarks();
  const [selected, setSelected] = useState<string | null>(null);
  // Default to the first uploaded symbol until the user picks one explicitly.
  const symbol = selected ?? benchmarks?.[0]?.symbol ?? null;

  const {
    data: benchmarkData,
    isLoading: benchmarkLoading,
    error: benchmarkError,
  } = useBenchmarkTimeseries(symbol);

  if (benchmarksLoading) {
    return <Skeleton className="h-9 w-40 rounded-md" />;
  }

  const hasBenchmarks = (benchmarks?.length ?? 0) > 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold">Benchmark Comparison</h2>
        <div className="flex items-center gap-2">
          {hasBenchmarks && symbol && (
            <Select value={symbol} onValueChange={setSelected}>
              <SelectTrigger className="h-8 w-32 text-sm">
                <SelectValue placeholder="Index" />
              </SelectTrigger>
              <SelectContent>
                {benchmarks!.map((b) => (
                  <SelectItem key={b.symbol} value={b.symbol}>
                    {b.symbol}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <BenchmarkUploadDialog />
        </div>
      </div>

      {!hasBenchmarks && (
        <p className="text-sm text-muted-foreground">
          No index price data uploaded yet — add a benchmark (e.g. an S&amp;P
          500 CSV export) to compare it against your portfolio&apos;s return.
        </p>
      )}

      {hasBenchmarks && benchmarkError && (
        <ApiErrorState error={benchmarkError} compact />
      )}

      {hasBenchmarks && benchmarkLoading && (
        <div className="grid gap-6 lg:grid-cols-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-64 w-full rounded-lg" />
          ))}
        </div>
      )}

      {hasBenchmarks && symbol && benchmarkData && benchmarkData.length > 0 && (
        <div className="grid gap-6 lg:grid-cols-2">
          <BenchmarkComparisonChart data={benchmarkData} symbol={symbol} />
          <TwrVsBenchmarkChart data={benchmarkData} symbol={symbol} />
        </div>
      )}
    </div>
  );
}
