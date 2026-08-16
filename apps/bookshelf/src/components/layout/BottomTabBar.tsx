"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Bell } from "lucide-react";
import { cn } from "@/lib/utils";
import { useUnreadNotifications } from "@/hooks/useUnreadNotifications";
import {
  primaryNavItems,
  profileNavItem,
  type NavItem,
} from "@/components/layout/navItems";

// Primary in-app navigation, thumb-reachable on mobile — replaces the old
// hamburger-menu pattern, which hides nav behind an extra tap.
export function BottomTabBar({ isAdmin }: { isAdmin: boolean }) {
  const pathname = usePathname();
  const unreadCount = useUnreadNotifications();

  const notificationsItem: NavItem = {
    href: "/notifications",
    label: "Alerts",
    icon: Bell,
    isActive: (p) => p === "/notifications",
  };

  const tabs: (NavItem & { badge?: number })[] = [
    ...primaryNavItems.filter((item) => item.mobileTab !== false),
    { ...notificationsItem, badge: unreadCount },
    profileNavItem(isAdmin),
  ];

  return (
    <nav
      className="md:hidden fixed inset-x-0 bottom-0 z-50 border-t bg-background/95 backdrop-blur"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <div className="grid grid-cols-5">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const active = tab.isActive(pathname);
          return (
            <Link
              key={tab.href}
              href={tab.href}
              className={cn(
                "flex flex-col items-center justify-center gap-0.5 py-2 text-[10px] font-medium transition-colors",
                active
                  ? "text-primary"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              <span className="relative">
                <Icon className="size-5" />
                {!!tab.badge && tab.badge > 0 && (
                  <span className="absolute -top-1 -right-2 flex size-3.5 items-center justify-center rounded-full bg-destructive text-[8px] font-bold text-white leading-none">
                    {tab.badge > 9 ? "9+" : tab.badge}
                  </span>
                )}
              </span>
              {tab.shortLabel ?? tab.label}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
