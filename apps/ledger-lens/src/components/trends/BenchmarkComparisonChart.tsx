"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { BenchmarkTimeseriesItem } from "@/lib/types";

interface Props {
  data: BenchmarkTimeseriesItem[];
  symbol: string;
}

interface RebasedPoint {
  year: string;
  portfolio: number | null;
  benchmark: number | null;
}

/** Rebase both cumulative indices to 100 at the first year with full benchmark coverage. */
function rebase(data: BenchmarkTimeseriesItem[]): RebasedPoint[] {
  const anchorIdx = data.findIndex((d) => d.coverage === "full");
  if (anchorIdx === -1) {
    return data.map((d) => ({
      year: d.year.toString(),
      portfolio: null,
      benchmark: null,
    }));
  }

  const portfolioBase = data[anchorIdx].portfolio_cum_index;
  const benchmarkBase = data[anchorIdx].benchmark_cum_index ?? 1;

  return data.map((d) => ({
    year: d.year.toString(),
    portfolio: (d.portfolio_cum_index / portfolioBase) * 100,
    benchmark:
      d.benchmark_cum_index !== null
        ? (d.benchmark_cum_index / benchmarkBase) * 100
        : null,
  }));
}

export function BenchmarkComparisonChart({ data, symbol }: Props) {
  const chartData = rebase(data);
  const hasGaps = data.some((d) => d.coverage === "missing");

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Portfolio vs. {symbol} (rebased to 100)
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={220}>
          <LineChart
            data={chartData}
            margin={{ top: 4, right: 8, bottom: 4, left: 8 }}
          >
            <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
            <XAxis dataKey="year" tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} />
            <Tooltip
              formatter={(v, name) => [
                v === null ? "—" : Number(v).toFixed(1),
                name === "portfolio" ? "Portfolio" : symbol,
              ]}
            />
            <Line
              type="monotone"
              dataKey="portfolio"
              name="portfolio"
              stroke="#3b82f6"
              strokeWidth={2}
              dot={{ r: 3 }}
              connectNulls
            />
            <Line
              type="monotone"
              dataKey="benchmark"
              name="benchmark"
              stroke="#a855f7"
              strokeWidth={2}
              strokeDasharray="4 3"
              dot={{ r: 3 }}
              connectNulls
            />
          </LineChart>
        </ResponsiveContainer>
        {hasGaps && (
          <p className="mt-2 text-xs text-muted-foreground">
            Some years have no {symbol} price coverage yet — those points are
            shown as gaps, not guessed at.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
