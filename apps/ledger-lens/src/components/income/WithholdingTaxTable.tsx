import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { WithholdingTaxItem } from "@/lib/types";
import { fmtUsd, fmtDate } from "@/lib/formatters";

interface Props {
  items: WithholdingTaxItem[];
}

export function WithholdingTaxTable({ items }: Props) {
  if (items.length === 0) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        No withholding tax recorded.
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
            <TableHead>Currency</TableHead>
            <TableHead className="text-right">Amount</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((t, i) => (
            <TableRow key={i}>
              <TableCell className="text-muted-foreground">
                {fmtDate(t.tax_date)}
              </TableCell>
              <TableCell className="font-medium">{t.symbol}</TableCell>
              <TableCell className="text-muted-foreground">
                {t.currency}
              </TableCell>
              <TableCell className="text-right font-mono text-red-500">
                {fmtUsd(t.amount)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
