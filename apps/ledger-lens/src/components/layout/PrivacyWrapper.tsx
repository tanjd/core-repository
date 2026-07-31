"use client";

import { usePrivacy } from "@/context/PrivacyContext";
import { cn } from "@/lib/utils";

export function PrivacyWrapper({ children }: { children: React.ReactNode }) {
  const { privacyMode } = usePrivacy();
  return (
    <div className={cn("contents", privacyMode && "privacy-on")}>
      {children}
    </div>
  );
}
