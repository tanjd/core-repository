import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type {
  DepositTimeseriesItem,
  DividendTimeseriesItem,
  NavTimeseriesItem,
  PnlTimeseriesItem,
} from "@/lib/types";
import { fmtUsd, fmtSgd, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import type { DcaItem } from "@/lib/types";

interface Props {
  navData: NavTimeseriesItem[];
  depositData: DepositTimeseriesItem[];
  dividendData: DividendTimeseriesItem[];
  pnlData: PnlTimeseriesItem[];
  dcaData?: DcaItem[];
  hasMoomoo?: boolean;
}

/** Compound annual TWR: ∏(1 + twr_i / 100) − 1, expressed as % */
function computeCumulativeTwr(items: NavTimeseriesItem[]): number {
  const product = items.reduce((acc, d) => acc * (1 + d.twr_pct / 100), 1);
  return (product - 1) * 100;
}

export function AllTimeSummaryCards({
  navData,
  depositData,
  dividendData,
  pnlData,
  dcaData,
  hasMoomoo,
}: Props) {
  const latestNav = navData.at(-1)?.nav_current ?? 0;
  const cumulativeTwr = computeCumulativeTwr(navData);
  const totalDeposits = depositData.at(-1)?.cumulative_deposits ?? 0;
  const totalNetDividends = dividendData.reduce((s, d) => s + d.net, 0);
  const totalRealized = pnlData.reduce((s, d) => s + d.realized, 0);
  const yearCount = navData.length;

  const latestUnrealized = pnlData.at(-1)?.unrealized ?? 0;
  const latestYear = navData.at(-1)?.year;

  // SGD deposit total comes from DCA data (native SGD amounts).
  // Rate = total SGD deposited / total USD equivalent of deposits.
  // totalDeposits is the USD-denominated cumulative deposit total from the deposits timeseries,
  // which already reflects the real USD equivalent of every SGD->USD conversion.
  const totalSgdDeposits = (dcaData ?? []).reduce((s, d) => s + d.sgd, 0);
  const impliedRate =
    totalDeposits > 0 && totalSgdDeposits > 0
      ? totalSgdDeposits / totalDeposits
      : null;

  const cards = [
    {
      title: "Current Portfolio Value",
      value: fmtUsd(latestNav),
      subtitle: impliedRate
        ? `≈ ${fmtSgd(latestNav * impliedRate)} · ${yearCount}yr`
        : `Across ${yearCount} year${yearCount !== 1 ? "s" : ""}`,
      valueClass: "",
    },
    {
      title: "Cumulative TWR",
      value: `${cumulativeTwr >= 0 ? "+" : ""}${cumulativeTwr.toFixed(2)}%`,
      subtitle: "Compounded across all years",
      valueClass: pnlColor(cumulativeTwr),
    },
    {
      title: "Total Deposits",
      value: fmtUsd(totalDeposits),
      subtitle: hasMoomoo
        ? "IBKR deposits + Moomoo est. from trades"
        : totalSgdDeposits > 0
          ? `${fmtSgd(totalSgdDeposits)} SGD`
          : "Cumulative USD",
      valueClass: "",
    },
    {
      title: "Total Net Dividends",
      value: fmtUsd(totalNetDividends),
      subtitle: "After withholding tax",
      valueClass: pnlColor(totalNetDividends),
    },
    {
      title: "Total Realized P&L",
      value: fmtUsd(totalRealized),
      subtitle: "All closed positions",
      valueClass: pnlColor(totalRealized),
    },
    {
      title: "Unrealized P&L",
      value: fmtUsd(latestUnrealized),
      subtitle: latestYear
        ? `Open positions (${latestYear})`
        : "Open positions",
      valueClass: pnlColor(latestUnrealized),
    },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-6">
      {cards.map((c) => (
        <Card key={c.title}>
          <CardHeader className="pb-1">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {c.title}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p
              data-val=""
              className={cn("text-xl font-bold tracking-tight", c.valueClass)}
            >
              {c.value}
            </p>
            {c.subtitle && (
              <p data-val="" className="mt-0.5 text-xs text-muted-foreground">
                {c.subtitle}
              </p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
