"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import type { TradeItem } from "@/lib/types";
import { fmtUsd, fmtDate, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const CODE_LABELS: Record<string, string> = {
  O: "Open",
  C: "Close",
  RI: "RI",
  FPA: "FPA",
  P: "Partial",
  Ca: "Cancel",
};

const BROKER_LABELS: Record<string, string> = {
  ibkr: "IBKR",
  moomoo: "Moomoo",
};

interface Props {
  trades: TradeItem[];
  currencyLabel?: string;
}

export function TradesTable({ trades, currencyLabel }: Props) {
  const sorted = [...trades].sort((a, b) =>
    b.trade_date.localeCompare(a.trade_date),
  );
  const multibroker = new Set(sorted.map((t) => t.broker)).size > 1;
  if (sorted.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-muted-foreground">
        No trades found.
      </p>
    );
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Date</TableHead>
            <TableHead>Symbol</TableHead>
            <TableHead>Side</TableHead>
            {multibroker && <TableHead>Broker</TableHead>}
            {currencyLabel && <TableHead>Currency</TableHead>}
            <TableHead className="text-right">Qty</TableHead>
            <TableHead className="text-right">Price</TableHead>
            <TableHead className="text-right">Proceeds</TableHead>
            <TableHead className="text-right">Commission</TableHead>
            <TableHead className="text-right">Realized P&L</TableHead>
            <TableHead>Codes</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((t) => (
            <TableRow key={t.id}>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {fmtDate(t.trade_date)}
              </TableCell>
              <TableCell className="font-medium">{t.symbol}</TableCell>
              <TableCell>
                <Badge
                  variant="outline"
                  className={
                    t.direction === "buy"
                      ? "border-green-600 text-green-600 dark:border-green-400 dark:text-green-400 text-xs"
                      : "border-red-500 text-red-500 dark:border-red-400 dark:text-red-400 text-xs"
                  }
                >
                  {t.direction === "buy" ? "Buy" : "Sell"}
                </Badge>
              </TableCell>
              {multibroker && (
                <TableCell>
                  <Badge variant="outline" className="text-xs">
                    {BROKER_LABELS[t.broker] ?? t.broker}
                  </Badge>
                </TableCell>
              )}
              {currencyLabel && (
                <TableCell className="text-muted-foreground">
                  {t.currency}
                </TableCell>
              )}
              <TableCell
                className={cn(
                  "text-right font-mono",
                  t.direction === "buy" ? "text-green-600" : "text-red-500",
                )}
              >
                {t.direction === "buy" ? "+" : ""}
                {t.quantity}
              </TableCell>
              <TableCell className="text-right font-mono">
                {fmtUsd(t.trade_price)}
              </TableCell>
              <TableCell className="text-right font-mono">
                {fmtUsd(t.proceeds)}
              </TableCell>
              <TableCell className="text-right font-mono text-muted-foreground">
                {fmtUsd(t.commission)}
              </TableCell>
              <TableCell
                className={cn("text-right font-mono", pnlColor(t.realized_pnl))}
              >
                {fmtUsd(t.realized_pnl)}
              </TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {t.codes.map((c) => (
                    <Badge
                      key={c}
                      variant="secondary"
                      className="text-xs px-1 py-0"
                    >
                      {CODE_LABELS[c] ?? c}
                    </Badge>
                  ))}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
