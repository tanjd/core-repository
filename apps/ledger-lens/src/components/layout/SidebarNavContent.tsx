"use client";

import { useRouter, usePathname } from "next/navigation";
import { useBrokerInfo } from "@/hooks/useStatement";
import { useBroker } from "@/context/BrokerContext";
import {
  BarChart3,
  BookOpen,
  CreditCard,
  DollarSign,
  History,
  Home,
  LineChart,
  TrendingUp,
} from "lucide-react";
import { cn } from "@/lib/utils";

const COMBINED_NAV = [
  { href: "/overview", label: "Overview", icon: Home },
  { href: "/holdings", label: "Holdings", icon: BookOpen },
  { href: "/trades", label: "Trades", icon: CreditCard },
  { href: "/history", label: "Upload History", icon: History },
];

const FLAT_NAV = [
  { href: "/overview", label: "Overview", icon: Home },
  { href: "/holdings", label: "Holdings", icon: BookOpen },
  { href: "/trades", label: "Trades", icon: CreditCard },
  { href: "/income", label: "Income", icon: DollarSign },
  { href: "/cashflows", label: "Cash Flows", icon: BarChart3 },
  { href: "/performance", label: "P&L Analysis", icon: TrendingUp },
  { href: "/trends", label: "Trends", icon: LineChart },
  { href: "/history", label: "Upload History", icon: History },
];

const IBKR_NAV = [
  { href: "/overview", label: "Overview", icon: Home },
  { href: "/holdings", label: "Holdings", icon: BookOpen },
  { href: "/trades", label: "Trades", icon: CreditCard },
  { href: "/income", label: "Income", icon: DollarSign },
  { href: "/cashflows", label: "Cash Flows", icon: BarChart3 },
  { href: "/performance", label: "P&L Analysis", icon: TrendingUp },
  { href: "/trends", label: "Trends", icon: LineChart },
];

const MOOMOO_NAV = [
  { href: "/overview", label: "Overview", icon: Home },
  { href: "/holdings", label: "Holdings", icon: BookOpen },
  { href: "/trades", label: "Trades", icon: CreditCard },
];

const BROKER_COLORS: Record<
  string,
  { label: string; active: string; hover: string; dot: string; text: string }
> = {
  ibkr: {
    label: "text-blue-500",
    text: "text-blue-600/75 dark:text-blue-400/70",
    active: "bg-blue-500/10 text-blue-600 dark:text-blue-400 font-medium",
    hover: "hover:bg-blue-500/10 hover:text-blue-600 dark:hover:text-blue-400",
    dot: "bg-blue-500",
  },
  moomoo: {
    label: "text-orange-500",
    text: "text-orange-600/75 dark:text-orange-400/70",
    active: "bg-orange-500/10 text-orange-600 dark:text-orange-400 font-medium",
    hover:
      "hover:bg-orange-500/10 hover:text-orange-600 dark:hover:text-orange-400",
    dot: "bg-orange-500",
  },
};

const BROKER_DISPLAY: Record<string, string> = {
  ibkr: "IBKR",
  moomoo: "Moomoo",
};

interface Props {
  onNavigate?: () => void;
}

export function SidebarNavContent({ onNavigate }: Props) {
  const pathname = usePathname();
  const router = useRouter();
  const { brokerList, selectedBroker, setSelectedBroker } = useBroker();
  const multibroker = brokerList.length > 1;
  const { data: brokerInfoList } = useBrokerInfo();

  function navigate(href: string, broker: string | null) {
    setSelectedBroker(broker);
    router.push(href);
    onNavigate?.();
  }

  return (
    <nav className="flex-1 overflow-y-auto p-2">
      {multibroker ? (
        <>
          {/* Combined top-level nav */}
          <div className="space-y-0.5">
            {COMBINED_NAV.map(({ href, label, icon: Icon }) => (
              <button
                key={`combined-${href}`}
                onClick={() => navigate(href, null)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
                  pathname === href && selectedBroker === null
                    ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                    : "text-sidebar-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                {label}
              </button>
            ))}
          </div>

          {/* Per-broker sections */}
          {brokerList.map((broker) => {
            const navItems = broker === "moomoo" ? MOOMOO_NAV : IBKR_NAV;
            const colors = BROKER_COLORS[broker];
            const info = brokerInfoList?.find((b) => b.broker === broker);
            const dataThrough = info?.latest_period_end
              ? new Date(
                  info.latest_period_end + "T00:00:00",
                ).toLocaleDateString(undefined, {
                  month: "short",
                  day: "numeric",
                  year: "numeric",
                })
              : null;
            return (
              <div key={broker} className="mt-3">
                <div className="mb-1 px-3 py-1">
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        "h-2 w-2 rounded-full shrink-0",
                        colors?.dot ?? "bg-muted-foreground",
                      )}
                    />
                    <span
                      className={cn(
                        "text-xs font-semibold uppercase tracking-wider",
                        colors?.label ?? "text-muted-foreground",
                      )}
                    >
                      {BROKER_DISPLAY[broker] ?? broker}
                    </span>
                  </div>
                  {dataThrough && (
                    <p className="mt-0.5 pl-4 text-[10px] text-muted-foreground/60">
                      data through {dataThrough}
                    </p>
                  )}
                </div>

                <div className="space-y-0.5">
                  {navItems.map(({ href, label, icon: Icon }) => {
                    const active =
                      pathname === href && selectedBroker === broker;
                    return (
                      <button
                        key={`${broker}-${href}`}
                        onClick={() => navigate(href, broker)}
                        className={cn(
                          "flex w-full items-center gap-2.5 rounded-md px-3 py-1.5 text-sm transition-colors",
                          active
                            ? (colors?.active ??
                                "bg-sidebar-accent text-sidebar-accent-foreground font-medium")
                            : cn(
                                colors?.text ?? "text-sidebar-foreground",
                                colors?.hover ?? "hover:bg-sidebar-accent/60",
                              ),
                        )}
                      >
                        <Icon className="h-4 w-4 shrink-0" />
                        {label}
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </>
      ) : (
        /* Single broker: flat nav */
        <div className="space-y-0.5">
          {FLAT_NAV.map(({ href, label, icon: Icon }) => {
            const active = pathname === href || pathname.startsWith(href + "/");
            return (
              <button
                key={href}
                onClick={() => navigate(href, null)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
                  active
                    ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                    : "text-sidebar-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                )}
              >
                <Icon className="h-4 w-4 shrink-0" />
                {label}
              </button>
            );
          })}
        </div>
      )}
    </nav>
  );
}
