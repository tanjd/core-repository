"use client";

import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
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

export function TwrVsBenchmarkChart({ data, symbol }: Props) {
  const chartData = data.map((d) => ({
    year: d.year.toString(),
    Portfolio: parseFloat(d.twr_pct.toFixed(2)),
    [symbol]:
      d.benchmark_return_pct !== null
        ? parseFloat(d.benchmark_return_pct.toFixed(2))
        : null,
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Portfolio Return vs. {symbol} by Year
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={220}>
          <BarChart
            data={chartData}
            margin={{ top: 4, right: 8, bottom: 4, left: 8 }}
          >
            <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
            <XAxis dataKey="year" tick={{ fontSize: 11 }} />
            <YAxis
              tick={{ fontSize: 11 }}
              tickFormatter={(v) => `${Number(v ?? 0)}%`}
            />
            <Tooltip
              formatter={(v) =>
                v === null ? ["No data"] : [`${Number(v ?? 0).toFixed(2)}%`]
              }
            />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            <Bar dataKey="Portfolio" fill="#3b82f6" radius={[4, 4, 0, 0]} />
            <Bar dataKey={symbol} fill="#a855f7" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
