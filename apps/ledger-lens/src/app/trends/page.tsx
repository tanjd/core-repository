"use client";

import { useNavTimeseries } from "@/hooks/useStatement";
import { PortfolioGrowthChart } from "@/components/trends/PortfolioGrowthChart";
import { TwrByYearChart } from "@/components/trends/TwrByYearChart";
import { Skeleton } from "@/components/ui/skeleton";

export default function TrendsPage() {
  const { data: navData, isLoading: navLoading } = useNavTimeseries();

  if (navLoading) {
    return (
      <div className="grid gap-6 lg:grid-cols-2">
        {Array.from({ length: 2 }).map((_, i) => (
          <Skeleton key={i} className="h-64 w-full rounded-lg" />
        ))}
      </div>
    );
  }

  const hasData = (navData?.length ?? 0) > 0;

  if (!hasData) {
    return (
      <p className="text-muted-foreground">
        Multi-year trends will appear here once you have imported data for more
        than one year.
      </p>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Trends</h1>
      <div className="grid gap-6 lg:grid-cols-2">
        {navData && <PortfolioGrowthChart data={navData} />}
        {navData && <TwrByYearChart data={navData} />}
      </div>
    </div>
  );
}
