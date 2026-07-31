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
import type { DepositTimeseriesItem } from "@/lib/types";
import { fmtUsd } from "@/lib/formatters";

interface Props {
  data: DepositTimeseriesItem[];
}

export function DepositVsGrowthChart({ data }: Props) {
  const chartData = data.map((d) => ({
    year: d.year.toString(),
    Deposits: d.total_deposits,
    Cumulative: d.cumulative_deposits,
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Deposits Over Time
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
            <Bar dataKey="Deposits" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            <Bar dataKey="Cumulative" fill="#8b5cf6" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
