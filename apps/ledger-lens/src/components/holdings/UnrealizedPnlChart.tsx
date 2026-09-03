"use client";

import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  LabelList,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { PositionItem } from "@/lib/types";
import { fmtUsd } from "@/lib/formatters";

interface Props {
  positions: PositionItem[];
}

export function UnrealizedPnlChart({ positions }: Props) {
  if (positions.length === 0) return null;

  // Merge same-symbol positions (e.g. same stock held in multiple brokers)
  const bySymbol = new Map<string, number>();
  for (const p of positions) {
    bySymbol.set(p.symbol, (bySymbol.get(p.symbol) ?? 0) + p.unrealized_pnl);
  }
  const chartData = [...bySymbol.entries()]
    .map(([symbol, pnl]) => ({
      symbol,
      "Unrealized P&L": parseFloat(pnl.toFixed(2)),
    }))
    .sort((a, b) => b["Unrealized P&L"] - a["Unrealized P&L"]);

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Unrealized P&amp;L by Position
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer
          width="100%"
          height={Math.max(180, chartData.length * 44)}
        >
          <BarChart
            data={chartData}
            layout="vertical"
            margin={{ top: 4, right: 16, bottom: 4, left: 8 }}
          >
            <CartesianGrid
              strokeDasharray="3 3"
              className="stroke-border"
              horizontal={false}
            />
            <XAxis
              type="number"
              tick={{ fontSize: 11 }}
              tickFormatter={(v) => fmtUsd(Number(v ?? 0))}
            />
            <YAxis
              type="category"
              dataKey="symbol"
              tick={{ fontSize: 12 }}
              width={56}
            />
            <Tooltip
              formatter={(v, n) => [fmtUsd(Number(v ?? 0)), String(n ?? "")]}
            />
            <Bar dataKey="Unrealized P&L" radius={[0, 4, 4, 0]}>
              {chartData.map((d, i) => (
                <Cell
                  key={i}
                  fill={d["Unrealized P&L"] >= 0 ? "#22c55e" : "#ef4444"}
                />
              ))}
              <LabelList
                dataKey="Unrealized P&L"
                position="right"
                fontSize={11}
                formatter={(v) => {
                  const n = Number(v ?? 0);
                  return `${n >= 0 ? "+" : ""}${fmtUsd(n)}`;
                }}
              />
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
