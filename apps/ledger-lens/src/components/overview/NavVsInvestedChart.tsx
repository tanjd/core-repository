"use client";

import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DepositTimeseriesItem, NavTimeseriesItem } from "@/lib/types";
import { fmtUsd } from "@/lib/formatters";

interface Props {
  navData: NavTimeseriesItem[];
  depositData: DepositTimeseriesItem[];
  hasMoomoo?: boolean;
}

export function NavVsInvestedChart({ navData, depositData, hasMoomoo }: Props) {
  const depositMap = new Map(
    depositData.map((d) => [d.year, d.cumulative_deposits]),
  );

  const chartData = navData.map((nav) => ({
    year: String(nav.year),
    "Portfolio NAV": nav.nav_current,
    "Total Invested": depositMap.get(nav.year) ?? 0,
  }));

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Portfolio Value vs Total Invested
          {hasMoomoo && (
            <span className="ml-2 text-xs font-normal text-muted-foreground">
              · Moomoo est. from trades
            </span>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={240}>
          <LineChart
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
            <Line
              type="monotone"
              dataKey="Portfolio NAV"
              stroke="#3b82f6"
              strokeWidth={2}
              dot={{ r: 4 }}
              activeDot={{ r: 6 }}
            />
            <Line
              type="monotone"
              dataKey="Total Invested"
              stroke="#8b5cf6"
              strokeWidth={2}
              strokeDasharray="5 5"
              dot={{ r: 4 }}
              activeDot={{ r: 6 }}
            />
          </LineChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
