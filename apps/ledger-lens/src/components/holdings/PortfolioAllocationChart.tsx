"use client";

import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { PositionItem } from "@/lib/types";
import { fmtUsd } from "@/lib/formatters";

const COLORS = [
  "#3b82f6",
  "#22c55e",
  "#f59e0b",
  "#8b5cf6",
  "#ec4899",
  "#14b8a6",
  "#f97316",
  "#6366f1",
];

interface Props {
  positions: PositionItem[];
  title?: string;
}

export function PortfolioAllocationChart({
  positions,
  title = "Portfolio Allocation",
}: Props) {
  // Merge same-symbol positions (e.g. same stock held in multiple brokers)
  const bySymbol = new Map<string, number>();
  for (const p of positions) {
    if (p.current_value > 0) {
      bySymbol.set(p.symbol, (bySymbol.get(p.symbol) ?? 0) + p.current_value);
    }
  }
  const data = [...bySymbol.entries()]
    .map(([symbol, value]) => ({ symbol, value }))
    .sort((a, b) => b.value - a.value);

  const total = data.reduce((s, d) => s + d.value, 0);

  if (data.length === 0) return null;

  return (
    <Card className="flex flex-col">
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col items-center gap-4 md:flex-row">
        <ResponsiveContainer width="100%" height={160}>
          <PieChart>
            <Pie
              data={data}
              dataKey="value"
              nameKey="symbol"
              cx="50%"
              cy="50%"
              innerRadius={45}
              outerRadius={70}
              paddingAngle={2}
            >
              {data.map((_, i) => (
                <Cell key={i} fill={COLORS[i % COLORS.length]} />
              ))}
            </Pie>
            <Tooltip
              formatter={(v) => fmtUsd(Number(v ?? 0))}
              contentStyle={{ fontSize: 12 }}
            />
          </PieChart>
        </ResponsiveContainer>
        <div className="w-full space-y-1.5 text-sm">
          {data.map((d, i) => (
            <div key={d.symbol} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span
                  className="inline-block h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ background: COLORS[i % COLORS.length] }}
                />
                <span className="text-muted-foreground">{d.symbol}</span>
              </div>
              <div className="text-right">
                <span data-val="" className="font-medium">
                  {fmtUsd(d.value)}
                </span>
                <span
                  data-val=""
                  className="ml-2 text-xs text-muted-foreground"
                >
                  {total > 0 ? ((d.value / total) * 100).toFixed(1) : "0"}%
                </span>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
