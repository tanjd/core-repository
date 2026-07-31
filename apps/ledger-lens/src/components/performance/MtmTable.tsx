import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { MtmItem } from "@/lib/types";
import { fmtUsd, fmtQty, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  items: MtmItem[];
}

export function MtmTable({ items }: Props) {
  if (items.length === 0) {
    return (
      <p className="py-4 text-sm text-muted-foreground">No MTM records.</p>
    );
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Symbol</TableHead>
            <TableHead className="text-right">Prior Qty</TableHead>
            <TableHead className="text-right">Current Qty</TableHead>
            <TableHead className="text-right">Prior Price</TableHead>
            <TableHead className="text-right">Current Price</TableHead>
            <TableHead className="text-right">MTM Total</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((m) => (
            <TableRow key={m.symbol}>
              <TableCell className="font-medium">{m.symbol}</TableCell>
              <TableCell className="text-right font-mono text-muted-foreground">
                {fmtQty(m.prior_qty)}
              </TableCell>
              <TableCell className="text-right font-mono">
                {fmtQty(m.current_qty)}
              </TableCell>
              <TableCell className="text-right font-mono text-muted-foreground">
                {fmtUsd(m.prior_price)}
              </TableCell>
              <TableCell className="text-right font-mono">
                {fmtUsd(m.current_price)}
              </TableCell>
              <TableCell
                className={cn(
                  "text-right font-mono font-medium",
                  pnlColor(m.mtm_total),
                )}
              >
                {fmtUsd(m.mtm_total)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
