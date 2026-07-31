import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { DepositItem } from "@/lib/types";
import { fmtCurrency, fmtDate, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  deposits: DepositItem[];
}

interface RowData {
  d: DepositItem;
  runningTotal: number;
}

function buildRows(deposits: DepositItem[]): RowData[] {
  return deposits.reduce<RowData[]>((acc, d) => {
    const prev = acc.length > 0 ? acc[acc.length - 1]!.runningTotal : 0;
    return [...acc, { d, runningTotal: prev + d.amount }];
  }, []);
}

export function DepositsTable({ deposits }: Props) {
  const rows = buildRows(deposits);

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Date</TableHead>
            <TableHead>Currency</TableHead>
            <TableHead>Description</TableHead>
            <TableHead className="text-right">Amount</TableHead>
            <TableHead className="text-right">Running Total</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map(({ d, runningTotal }, i) => (
            <TableRow key={i}>
              <TableCell className="text-muted-foreground">
                {fmtDate(d.settle_date)}
              </TableCell>
              <TableCell>{d.currency}</TableCell>
              <TableCell className="max-w-[200px] truncate text-muted-foreground">
                {d.description}
              </TableCell>
              <TableCell
                className={cn("text-right font-mono", pnlColor(d.amount))}
              >
                {fmtCurrency(d.amount, d.currency)}
              </TableCell>
              <TableCell className="text-right font-mono text-muted-foreground">
                {fmtCurrency(runningTotal, d.currency)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
