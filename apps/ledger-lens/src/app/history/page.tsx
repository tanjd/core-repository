"use client";

import { AlertCircle, CheckCircle2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { fmtUsd, fmtPct } from "@/lib/formatters";
import { useUploadHistory } from "@/hooks/useStatement";
import type { UploadLogItem } from "@/lib/types";

const BROKER_LABELS: Record<string, string> = {
  ibkr: "IBKR",
  moomoo: "Moomoo",
  unknown: "Unknown",
};

function countsLabel(item: UploadLogItem): string {
  const parts: string[] = [];
  if (item.trade_count > 0) parts.push(`${item.trade_count} trades`);
  if (item.position_count > 0) parts.push(`${item.position_count} positions`);
  if (item.deposit_count > 0) parts.push(`${item.deposit_count} deposits`);
  if (item.dividend_count > 0) parts.push(`${item.dividend_count} dividends`);
  return parts.join(" · ") || "—";
}

function formatUploadedAt(isoStr: string): string {
  const d = new Date(isoStr + "Z"); // treat as UTC
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function HistoryPage() {
  const { data: logs, isLoading } = useUploadHistory();

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">Upload History</h1>
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded" />
          ))}
        </div>
      </div>
    );
  }

  if (!logs || logs.length === 0) {
    return (
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">Upload History</h1>
        <p className="text-muted-foreground">
          No uploads yet. Import a CSV to see the history here.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Upload History</h1>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>When</TableHead>
              <TableHead>File</TableHead>
              <TableHead>Broker</TableHead>
              <TableHead>Account</TableHead>
              <TableHead>Year</TableHead>
              <TableHead className="text-right">NAV</TableHead>
              <TableHead>Counts</TableHead>
              <TableHead>Source</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                  {formatUploadedAt(item.uploaded_at)}
                </TableCell>
                <TableCell
                  className="max-w-48 truncate text-sm font-mono"
                  title={item.filename}
                >
                  {item.filename}
                </TableCell>
                <TableCell className="text-sm">
                  {BROKER_LABELS[item.broker] ?? item.broker}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {item.account_id || "—"}
                </TableCell>
                <TableCell className="text-sm">
                  {item.year > 0 ? item.year : "—"}
                </TableCell>
                <TableCell className="text-right text-sm">
                  {item.nav_current > 0 ? (
                    <span>
                      {fmtUsd(item.nav_current)}
                      {item.twr_pct !== 0 && (
                        <span className="ml-1 text-xs text-muted-foreground">
                          {fmtPct(item.twr_pct)} TWR
                        </span>
                      )}
                    </span>
                  ) : (
                    "—"
                  )}
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {item.status === "success" ? countsLabel(item) : "—"}
                </TableCell>
                <TableCell>
                  <Badge
                    variant={item.source === "upload" ? "default" : "secondary"}
                  >
                    {item.source}
                  </Badge>
                </TableCell>
                <TableCell>
                  {item.status === "success" ? (
                    <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400" />
                  ) : (
                    <span title={item.error_msg ?? "Error"}>
                      <AlertCircle className="h-4 w-4 text-destructive" />
                    </span>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
