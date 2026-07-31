"use client";

import { Eye, EyeOff, Menu } from "lucide-react";
import { useYear } from "@/context/YearContext";
import { usePrivacy } from "@/context/PrivacyContext";
import { useMobileNav } from "@/context/MobileNavContext";
import { useBroker } from "@/context/BrokerContext";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { UploadDialog } from "./UploadDialog";
import { cn } from "@/lib/utils";

export function TopBar() {
  const { years, selectedYear, setSelectedYear } = useYear();
  const { privacyMode, togglePrivacyMode } = usePrivacy();
  const { openNav } = useMobileNav();
  const { selectedBroker } = useBroker();

  return (
    <header
      className={cn(
        "flex h-14 items-center justify-between border-b px-4",
        selectedBroker === "ibkr" && "border-blue-500/50",
        selectedBroker === "moomoo" && "border-orange-500/50",
      )}
    >
      <button
        className="md:hidden rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground"
        onClick={openNav}
        aria-label="Open navigation"
      >
        <Menu className="h-5 w-5" />
      </button>
      <div className="hidden md:flex items-center">
        {selectedBroker === "ibkr" && (
          <span className="text-xs font-semibold px-2 py-0.5 rounded-md text-blue-600 dark:text-blue-400 bg-blue-500/10">
            IBKR
          </span>
        )}
        {selectedBroker === "moomoo" && (
          <span className="text-xs font-semibold px-2 py-0.5 rounded-md text-orange-600 dark:text-orange-400 bg-orange-500/10">
            Moomoo
          </span>
        )}
      </div>
      <div className="flex items-center gap-3">
        {years.length > 0 && (
          <Select
            value={selectedYear?.toString() ?? ""}
            onValueChange={(v) => setSelectedYear(Number(v))}
          >
            <SelectTrigger className="h-8 w-28 text-sm">
              <SelectValue placeholder="Year" />
            </SelectTrigger>
            <SelectContent>
              {[...years]
                .sort((a, b) => b - a)
                .map((y) => (
                  <SelectItem key={y} value={y.toString()}>
                    {y}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={togglePrivacyMode}
          title={privacyMode ? "Show values" : "Hide values"}
        >
          {privacyMode ? (
            <EyeOff className="h-4 w-4 text-muted-foreground" />
          ) : (
            <Eye className="h-4 w-4 text-muted-foreground" />
          )}
        </Button>
        <UploadDialog />
      </div>
    </header>
  );
}
