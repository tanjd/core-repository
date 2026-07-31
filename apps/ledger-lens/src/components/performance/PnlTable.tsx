import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { PnlItem, PnlSummary } from "@/lib/types";
import { fmtUsd, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  records: PnlItem[];
  summary: PnlSummary;
}

export function PnlTable({ records, summary }: Props) {
  if (records.length === 0) {
    return (
      <p className="py-4 text-sm text-muted-foreground">No P&L records.</p>
    );
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Symbol</TableHead>
            <TableHead>Category</TableHead>
            <TableHead className="text-right">ST Realized</TableHead>
            <TableHead className="text-right">LT Realized</TableHead>
            <TableHead className="text-right">Unrealized</TableHead>
            <TableHead className="text-right">Total</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {records.map((r) => (
            <TableRow key={r.symbol}>
              <TableCell className="font-medium">{r.symbol}</TableCell>
              <TableCell className="text-muted-foreground">
                {r.asset_category}
              </TableCell>
              <TableCell
                className={cn(
                  "text-right font-mono",
                  pnlColor(r.realized_st_profit + r.realized_st_loss),
                )}
              >
                {fmtUsd(r.realized_st_profit + r.realized_st_loss)}
              </TableCell>
              <TableCell
                className={cn(
                  "text-right font-mono",
                  pnlColor(r.realized_lt_profit + r.realized_lt_loss),
                )}
              >
                {fmtUsd(r.realized_lt_profit + r.realized_lt_loss)}
              </TableCell>
              <TableCell
                className={cn(
                  "text-right font-mono",
                  pnlColor(r.unrealized_total),
                )}
              >
                {fmtUsd(r.unrealized_total)}
              </TableCell>
              <TableCell
                className={cn(
                  "text-right font-mono font-medium",
                  pnlColor(r.total),
                )}
              >
                {fmtUsd(r.total)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <div className="flex justify-end gap-8 border-t px-4 py-3 text-sm font-semibold">
        <span>
          ST Realized:{" "}
          <span className={cn("ml-1 font-mono", pnlColor(summary.realized_st))}>
            {fmtUsd(summary.realized_st)}
          </span>
        </span>
        <span>
          LT Realized:{" "}
          <span className={cn("ml-1 font-mono", pnlColor(summary.realized_lt))}>
            {fmtUsd(summary.realized_lt)}
          </span>
        </span>
        <span>
          Unrealized:{" "}
          <span
            className={cn("ml-1 font-mono", pnlColor(summary.unrealized_total))}
          >
            {fmtUsd(summary.unrealized_total)}
          </span>
        </span>
        <span>
          Total:{" "}
          <span className={cn("ml-1 font-mono", pnlColor(summary.total))}>
            {fmtUsd(summary.total)}
          </span>
        </span>
      </div>
    </div>
  );
}
