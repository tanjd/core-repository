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
import type { NavTimeseriesItem } from "@/lib/types";

interface Props {
  data: NavTimeseriesItem[];
}

export function TwrByYearChart({ data }: Props) {
  const chartData = data.map((d) => ({
    year: d.year.toString(),
    TWR: parseFloat(d.twr_pct.toFixed(2)),
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Time-Weighted Return by Year
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
              formatter={(v) => [`${Number(v ?? 0).toFixed(2)}%`, "TWR"]}
            />
            <Bar dataKey="TWR" radius={[4, 4, 0, 0]}>
              {chartData.map((d, i) => (
                <Cell key={i} fill={d.TWR >= 0 ? "#22c55e" : "#ef4444"} />
              ))}
              <LabelList
                dataKey="TWR"
                position="top"
                fontSize={11}
                formatter={(v) => {
                  const n = Number(v ?? 0);
                  return `${n >= 0 ? "+" : ""}${n}%`;
                }}
              />
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
