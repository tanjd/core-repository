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
import type { MonthlyCashflow } from "@/lib/types";
import { fmtMonth, fmtUsd } from "@/lib/formatters";

interface Props {
  data: MonthlyCashflow[];
}

export function CashFlowBarChart({ data }: Props) {
  if (data.length === 0) return null;

  const chartData = data.map((d) => ({
    month: fmtMonth(d.month),
    SGD: d.sgd,
    USD: d.usd,
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Monthly Cash Flows
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={220}>
          <BarChart
            data={chartData}
            margin={{ top: 4, right: 8, bottom: 4, left: 8 }}
          >
            <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
            <XAxis dataKey="month" tick={{ fontSize: 11 }} />
            <YAxis
              tick={{ fontSize: 11 }}
              tickFormatter={(v) => fmtUsd(Number(v ?? 0))}
            />
            <Tooltip
              formatter={(v, n) => [fmtUsd(Number(v ?? 0)), String(n ?? "")]}
            />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Bar dataKey="SGD" fill="#3b82f6" radius={[3, 3, 0, 0]} />
            <Bar dataKey="USD" fill="#22c55e" radius={[3, 3, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
