"use client";

import { useYear } from "@/context/YearContext";
import { useBroker } from "@/context/BrokerContext";
import { useHoldings } from "@/hooks/useStatement";
import { HoldingsTable } from "@/components/holdings/HoldingsTable";
import { PortfolioAllocationChart } from "@/components/holdings/PortfolioAllocationChart";
import { UnrealizedPnlChart } from "@/components/holdings/UnrealizedPnlChart";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import type { PositionItem } from "@/lib/types";

const BROKER_LABELS: Record<string, string> = {
  ibkr: "IBKR",
  moomoo: "Moomoo",
};

function mergePositions(positions: PositionItem[]): PositionItem[] {
  const map = new Map<string, PositionItem>();
  for (const pos of positions) {
    const existing = map.get(pos.symbol);
    if (!existing) {
      map.set(pos.symbol, { ...pos, broker: "combined" });
    } else {
      const qty = existing.quantity + pos.quantity;
      const basis = existing.cost_basis + pos.cost_basis;
      map.set(pos.symbol, {
        ...existing,
        quantity: qty,
        cost_basis: basis,
        cost_price: qty > 0 ? basis / qty : 0,
        close_price: pos.close_price,
        current_value: existing.current_value + pos.current_value,
        unrealized_pnl: existing.unrealized_pnl + pos.unrealized_pnl,
      });
    }
  }
  return [...map.values()];
}

function PositionView({ positions }: { positions: PositionItem[] }) {
  return (
    <div className="space-y-4 pt-4">
      <div className="grid gap-4 md:grid-cols-2">
        <PortfolioAllocationChart positions={positions} />
        <UnrealizedPnlChart positions={positions} />
      </div>
      <HoldingsTable positions={positions} />
    </div>
  );
}

export default function HoldingsPage() {
  const { selectedYear } = useYear();
  const { selectedBroker } = useBroker();
  const { data, isLoading } = useHoldings(selectedYear);

  if (!selectedYear) {
    return (
      <p className="text-muted-foreground">Select a year to view holdings.</p>
    );
  }

  if (isLoading || !data) {
    return <Skeleton className="h-64 w-full rounded-lg" />;
  }

  if (data.positions.length === 0) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">Holdings — {selectedYear}</h1>
        <p className="text-muted-foreground">
          No open positions for this year.
        </p>
      </div>
    );
  }

  const brokers = [...new Set(data.positions.map((p) => p.broker))];
  const multibroker = brokers.length > 1;

  // If a broker is selected globally, filter and show directly (no tabs)
  if (selectedBroker) {
    const filtered = data.positions.filter((p) => p.broker === selectedBroker);
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">
          Holdings — {selectedYear}
          <span className="ml-2 text-sm font-normal text-muted-foreground">
            · {BROKER_LABELS[selectedBroker] ?? selectedBroker}
          </span>
        </h1>
        {filtered.length === 0 ? (
          <p className="text-muted-foreground">
            No positions for this broker and year.
          </p>
        ) : (
          <PositionView positions={filtered} />
        )}
      </div>
    );
  }

  // Multi-broker: show tabs
  if (multibroker) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">Holdings — {selectedYear}</h1>
        <Tabs defaultValue="combined">
          <TabsList>
            <TabsTrigger value="combined">Combined</TabsTrigger>
            {brokers.map((b) => (
              <TabsTrigger key={b} value={b}>
                {BROKER_LABELS[b] ?? b}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="combined">
            <PositionView positions={mergePositions(data.positions)} />
          </TabsContent>
          {brokers.map((b) => (
            <TabsContent key={b} value={b}>
              <PositionView
                positions={data.positions.filter((p) => p.broker === b)}
              />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    );
  }

  // Single broker
  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Holdings — {selectedYear}</h1>
      <PositionView positions={data.positions} />
    </div>
  );
}
