import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  DepositTimeseriesItem,
  DividendTimeseriesItem,
  NavTimeseriesItem,
  PnlTimeseriesItem,
} from "@/lib/types";
import { fmtUsd, fmtPct, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  navData: NavTimeseriesItem[];
  depositData: DepositTimeseriesItem[];
  dividendData: DividendTimeseriesItem[];
  pnlData: PnlTimeseriesItem[];
  selectedYear: number | null;
  onSelectYear: (year: number) => void;
}

interface YearRow {
  year: number;
  navCurrent: number;
  twrPct: number;
  deposits: number;
  netDividends: number;
  realizedPnl: number;
}

function buildRows(
  navData: NavTimeseriesItem[],
  depositData: DepositTimeseriesItem[],
  dividendData: DividendTimeseriesItem[],
  pnlData: PnlTimeseriesItem[],
): YearRow[] {
  const depositMap = new Map(
    depositData.map((d) => [d.year, d.total_deposits]),
  );
  const dividendMap = new Map(dividendData.map((d) => [d.year, d.net]));
  const pnlMap = new Map(pnlData.map((d) => [d.year, d.realized]));

  return [...navData]
    .sort((a, b) => b.year - a.year)
    .map((n) => ({
      year: n.year,
      navCurrent: n.nav_current,
      twrPct: n.twr_pct,
      deposits: depositMap.get(n.year) ?? 0,
      netDividends: dividendMap.get(n.year) ?? 0,
      realizedPnl: pnlMap.get(n.year) ?? 0,
    }));
}

export function YearSummaryTable({
  navData,
  depositData,
  dividendData,
  pnlData,
  selectedYear,
  onSelectYear,
}: Props) {
  const rows = buildRows(navData, depositData, dividendData, pnlData);

  if (rows.length === 0) return null;

  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Year-by-Year Summary
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Year</TableHead>
              <TableHead className="text-right">Year-End NAV</TableHead>
              <TableHead className="text-right">TWR</TableHead>
              <TableHead className="text-right">Deposits</TableHead>
              <TableHead className="text-right">Net Dividends</TableHead>
              <TableHead className="text-right">Realized P&L</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow
                key={r.year}
                className={cn(
                  "cursor-pointer transition-colors hover:bg-muted/50",
                  r.year === selectedYear && "bg-muted/60 font-medium",
                )}
                onClick={() => onSelectYear(r.year)}
              >
                <TableCell className="font-medium">{r.year}</TableCell>
                <TableCell className="text-right font-mono">
                  {fmtUsd(r.navCurrent)}
                </TableCell>
                <TableCell
                  className={cn("text-right font-mono", pnlColor(r.twrPct))}
                >
                  {fmtPct(r.twrPct)}
                </TableCell>
                <TableCell className="text-right font-mono">
                  {fmtUsd(r.deposits)}
                </TableCell>
                <TableCell
                  className={cn(
                    "text-right font-mono",
                    pnlColor(r.netDividends),
                  )}
                >
                  {fmtUsd(r.netDividends)}
                </TableCell>
                <TableCell
                  className={cn(
                    "text-right font-mono",
                    pnlColor(r.realizedPnl),
                  )}
                >
                  {fmtUsd(r.realizedPnl)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
