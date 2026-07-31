"use client";

import { createContext, useCallback, useContext, useState } from "react";

interface MobileNavContextValue {
  isOpen: boolean;
  openNav: () => void;
  closeNav: () => void;
}

const MobileNavContext = createContext<MobileNavContextValue>({
  isOpen: false,
  openNav: () => undefined,
  closeNav: () => undefined,
});

export function MobileNavProvider({ children }: { children: React.ReactNode }) {
  const [isOpen, setIsOpen] = useState(false);
  const openNav = useCallback(() => setIsOpen(true), []);
  const closeNav = useCallback(() => setIsOpen(false), []);

  return (
    <MobileNavContext.Provider value={{ isOpen, openNav, closeNav }}>
      {children}
    </MobileNavContext.Provider>
  );
}

export function useMobileNav() {
  return useContext(MobileNavContext);
}
