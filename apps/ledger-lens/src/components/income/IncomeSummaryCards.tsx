import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { IncomeSummary } from "@/lib/types";
import { fmtUsd, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  summary: IncomeSummary;
}

export function IncomeSummaryCards({ summary }: Props) {
  const cards = [
    { title: "Gross Dividends", value: summary.gross_dividends, colored: true },
    { title: "Withholding Tax", value: summary.withholding_tax, colored: true },
    { title: "Net Dividends", value: summary.net_dividends, colored: true },
    {
      title: "Fees & Comm. Adj.",
      value: summary.fees + summary.commission_adjustments,
      colored: true,
    },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
              className={cn(
                "text-2xl font-bold",
                c.colored ? pnlColor(c.value) : "",
              )}
            >
              {fmtUsd(c.value)}
            </p>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
