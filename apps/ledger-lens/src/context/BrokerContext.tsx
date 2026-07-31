"use client";

import { createContext, useCallback, useContext, useState } from "react";
import useSWR from "swr";
import { fetcher, BROKERS_URL } from "@/lib/api";

interface BrokerContextValue {
  brokerList: string[];
  selectedBroker: string | null;
  setSelectedBroker: (b: string | null) => void;
}

const BrokerContext = createContext<BrokerContextValue>({
  brokerList: [],
  selectedBroker: null,
  setSelectedBroker: () => undefined,
});

export function BrokerProvider({ children }: { children: React.ReactNode }) {
  const { data: brokerList = [] } = useSWR<string[]>(BROKERS_URL, fetcher);
  const [selectedBroker, setSelectedBrokerState] = useState<string | null>(
    null,
  );
  const setSelectedBroker = useCallback(
    (b: string | null) => setSelectedBrokerState(b),
    [],
  );

  return (
    <BrokerContext.Provider
      value={{ brokerList, selectedBroker, setSelectedBroker }}
    >
      {children}
    </BrokerContext.Provider>
  );
}

export function useBroker() {
  return useContext(BrokerContext);
}
