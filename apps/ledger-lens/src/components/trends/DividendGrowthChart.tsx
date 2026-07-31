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
import type { DividendTimeseriesItem } from "@/lib/types";
import { fmtUsd } from "@/lib/formatters";

interface Props {
  data: DividendTimeseriesItem[];
}

export function DividendGrowthChart({ data }: Props) {
  const chartData = data.map((d) => ({
    year: d.year.toString(),
    Gross: d.gross,
    Net: d.net,
    Withholding: Math.abs(d.withholding),
    Fees: Math.abs(d.fees),
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Annual Dividend Income
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
              tickFormatter={(v) => fmtUsd(Number(v ?? 0))}
            />
            <Tooltip
              formatter={(v, n) => [fmtUsd(Number(v ?? 0)), String(n ?? "")]}
            />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Bar dataKey="Gross" fill="#22c55e" radius={[3, 3, 0, 0]} />
            <Bar dataKey="Net" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            <Bar dataKey="Withholding" fill="#f59e0b" radius={[3, 3, 0, 0]} />
            <Bar dataKey="Fees" fill="#ef4444" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
