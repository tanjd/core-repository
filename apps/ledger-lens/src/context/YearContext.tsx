"use client";

import { createContext, useCallback, useContext, useState } from "react";
import useSWR, { mutate as globalMutate } from "swr";
import { fetcher } from "@/lib/api";

interface YearContextValue {
  years: number[];
  selectedYear: number | null;
  setSelectedYear: (year: number) => void;
  refreshYears: () => void;
}

const YearContext = createContext<YearContextValue>({
  years: [],
  selectedYear: null,
  setSelectedYear: () => undefined,
  refreshYears: () => undefined,
});

export function YearProvider({ children }: { children: React.ReactNode }) {
  const { data: years = [] } = useSWR<number[]>("/api/years", fetcher, {
    refreshInterval: 0,
  });

  // null means "auto" — will resolve to latest year
  const [manualYear, setManualYear] = useState<number | null>(null);

  // Derive effective selected year: manual pick (if still valid) or latest
  const latestYear = years.length > 0 ? Math.max(...years) : null;
  const selectedYear =
    manualYear !== null && years.includes(manualYear) ? manualYear : latestYear;

  const setSelectedYear = useCallback((year: number) => {
    setManualYear(year);
  }, []);

  const refreshYears = useCallback(() => {
    void globalMutate("/api/years");
  }, []);

  return (
    <YearContext.Provider
      value={{ years, selectedYear, setSelectedYear, refreshYears }}
    >
      {children}
    </YearContext.Provider>
  );
}

export function useYear() {
  return useContext(YearContext);
}
