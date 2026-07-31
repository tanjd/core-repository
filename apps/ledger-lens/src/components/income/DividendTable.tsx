import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { DividendItem } from "@/lib/types";
import { fmtUsd, fmtDate } from "@/lib/formatters";

interface Props {
  dividends: DividendItem[];
}

export function DividendTable({ dividends }: Props) {
  if (dividends.length === 0) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        No dividends recorded.
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
            <TableHead>Type</TableHead>
            <TableHead className="text-right">Per Share</TableHead>
            <TableHead className="text-right">Gross Amount</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {dividends.map((d, i) => (
            <TableRow key={i}>
              <TableCell className="text-muted-foreground">
                {fmtDate(d.pay_date)}
              </TableCell>
              <TableCell className="font-medium">{d.symbol}</TableCell>
              <TableCell className="text-muted-foreground text-sm">
                {d.dividend_type}
              </TableCell>
              <TableCell className="text-right font-mono">
                {fmtUsd(d.per_share_rate)}
              </TableCell>
              <TableCell className="text-right font-mono text-green-600">
                {fmtUsd(d.gross_amount)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
