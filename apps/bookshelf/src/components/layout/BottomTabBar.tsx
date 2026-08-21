"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Plus, ScanLine, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { primaryNavItems } from "@/components/layout/navItems";

const tabItemClass =
  "flex flex-col items-center justify-center gap-0.5 py-2 text-[10px] font-medium transition-colors";

// Primary in-app navigation, thumb-reachable on mobile — replaces the old
// hamburger-menu pattern, which hides nav behind an extra tap. "Share" is a
// raised popover trigger (Scan ISBN / Search) rather than a Link, centered
// among the other tabs — see apps/bookshelf/CLAUDE.md's "Mobile-first UI"
// section for the slot budget this bar follows.
export function BottomTabBar() {
  const pathname = usePathname();

  const tabs = primaryNavItems.filter((item) => item.mobileTab !== false);
  const firstHalf = tabs.slice(0, Math.ceil(tabs.length / 2));
  const secondHalf = tabs.slice(Math.ceil(tabs.length / 2));

  function renderTab(tab: (typeof tabs)[number]) {
    const Icon = tab.icon;
    const active = tab.isActive(pathname);
    return (
      <Link
        key={tab.href}
        href={tab.href}
        className={cn(
          tabItemClass,
          active
            ? "text-primary"
            : "text-muted-foreground hover:text-foreground",
        )}
      >
        <Icon className="size-5" />
        {tab.shortLabel ?? tab.label}
      </Link>
    );
  }

  return (
    <nav
      className="md:hidden fixed inset-x-0 bottom-0 z-50 border-t bg-background/95 backdrop-blur"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <div className="grid grid-cols-5">
        {firstHalf.map(renderTab)}

        <Popover>
          <PopoverTrigger asChild>
            <button
              aria-label="Share a book — scan or search"
              className="flex flex-col items-center justify-center gap-0.5 text-[10px] font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              <span className="-translate-y-3 flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg active:scale-95 transition-transform data-[state=open]:rotate-45">
                <Plus className="size-6" />
              </span>
              <span className="-mt-2">Share</span>
            </button>
          </PopoverTrigger>
          <PopoverContent
            side="top"
            align="center"
            sideOffset={12}
            className="md:hidden w-52 p-1"
          >
            <Link
              href="/share/scan"
              className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
            >
              <ScanLine className="size-4" />
              Scan ISBN
            </Link>
            <Link
              href="/share"
              className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
            >
              <Search className="size-4" />
              Search
            </Link>
          </PopoverContent>
        </Popover>

        {secondHalf.map(renderTab)}
      </div>
    </nav>
  );
}
