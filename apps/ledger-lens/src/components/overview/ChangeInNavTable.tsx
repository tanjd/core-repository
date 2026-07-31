import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ChangeInNav } from "@/lib/types";
import { fmtUsd, pnlColor } from "@/lib/formatters";
import { cn } from "@/lib/utils";

interface Props {
  data: ChangeInNav;
}

const ROWS: { label: string; key: keyof ChangeInNav }[] = [
  { label: "Starting value", key: "starting_value" },
  { label: "Deposits / withdrawals", key: "deposits_withdrawals" },
  { label: "Mark-to-market P&L", key: "mark_to_market" },
  { label: "Dividends", key: "dividends" },
  { label: "Withholding tax", key: "withholding_tax" },
  { label: "Commissions", key: "commissions" },
  { label: "Other fees", key: "other_fees" },
  { label: "Sales tax", key: "sales_tax" },
];

export function ChangeInNavTable({ data }: Props) {
  return (
    <Card>
      <CardHeader className="pb-1">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          Change in NAV
        </CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <table className="w-full text-sm">
          <tbody>
            {ROWS.map(({ label, key }) => {
              const v = data[key];
              return (
                <tr key={key} className="border-b last:border-0">
                  <td className="px-4 py-2 text-muted-foreground">{label}</td>
                  <td
                    className={cn(
                      "px-4 py-2 text-right font-mono",
                      key !== "starting_value" ? pnlColor(v) : "",
                    )}
                  >
                    {fmtUsd(v)}
                  </td>
                </tr>
              );
            })}
            <tr className="border-t-2 font-semibold">
              <td className="px-4 py-2">Ending value</td>
              <td className="px-4 py-2 text-right font-mono">
                {fmtUsd(data.ending_value)}
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
