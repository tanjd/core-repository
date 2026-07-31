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
import type { DcaItem } from "@/lib/types";

const MONTH_NAMES = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

// One color pair per year
const YEAR_PALETTE: Record<string, { sgd: string; usd: string }> = {
  "0": { sgd: "#3b82f6", usd: "#93c5fd" },
  "1": { sgd: "#22c55e", usd: "#86efac" },
  "2": { sgd: "#f59e0b", usd: "#fcd34d" },
  "3": { sgd: "#8b5cf6", usd: "#c4b5fd" },
};

interface Props {
  data: DcaItem[];
}

interface ChartPoint {
  label: string;
  [key: string]: string | number;
}

function buildChartData(items: DcaItem[]) {
  if (items.length === 0) return { points: [], seriesKeys: [] };

  const years = [...new Set(items.map((d) => d.year))].sort((a, b) => a - b);

  // Index by (year, month) for fast lookup
  const index = new Map(items.map((d) => [`${d.year}-${d.month}`, d]));

  // Build month rows (only months that have any data across any year)
  const activeMonths = new Set(items.map((d) => d.month));
  const sortedMonths = [...activeMonths].sort((a, b) => a - b);

  const points: ChartPoint[] = sortedMonths.map((m) => {
    const point: ChartPoint = { label: MONTH_NAMES[m - 1]! };
    for (const year of years) {
      const entry = index.get(`${year}-${m}`);
      const palette = YEAR_PALETTE[String(years.indexOf(year) % 4)]!;
      point[`${year} SGD`] = entry?.sgd ?? 0;
      point[`${year} USD`] = entry?.usd ?? 0;
      // Store colors for reference (not rendered directly)
      point[`_sgdColor_${year}`] = palette.sgd;
      point[`_usdColor_${year}`] = palette.usd;
    }
    return point;
  });

  // Series keys (e.g. ["2025 SGD", "2025 USD", "2024 SGD", "2024 USD"])
  const seriesKeys = years.flatMap((year) => {
    const i = years.indexOf(year) % 4;
    const palette = YEAR_PALETTE[String(i)]!;
    const hasSgd = items.some((d) => d.year === year && d.sgd > 0);
    const hasUsd = items.some((d) => d.year === year && d.usd > 0);
    const series: { key: string; fill: string }[] = [];
    if (hasSgd) series.push({ key: `${year} SGD`, fill: palette.sgd });
    if (hasUsd) series.push({ key: `${year} USD`, fill: palette.usd });
    return series;
  });

  return { points, seriesKeys };
}

function fmtAmount(v: number | undefined): string {
  return (v ?? 0).toLocaleString("en-US", { maximumFractionDigits: 0 });
}

export function MonthlyDepositsChart({ data }: Props) {
  const { points, seriesKeys } = buildChartData(data);

  if (points.length === 0) return null;

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Monthly Deposits by Year
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={240}>
          <BarChart
            data={points}
            margin={{ top: 4, right: 8, bottom: 4, left: 8 }}
          >
            <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
            <XAxis dataKey="label" tick={{ fontSize: 11 }} />
            <YAxis
              tick={{ fontSize: 11 }}
              tickFormatter={(v) => fmtAmount(Number(v ?? 0))}
            />
            <Tooltip
              formatter={(v, n) => [fmtAmount(Number(v ?? 0)), String(n ?? "")]}
            />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            {seriesKeys.map(({ key, fill }) => (
              <Bar
                key={key}
                dataKey={key}
                fill={fill}
                radius={[3, 3, 0, 0]}
                maxBarSize={32}
              />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
