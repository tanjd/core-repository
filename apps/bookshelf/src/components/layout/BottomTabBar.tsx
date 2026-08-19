"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Bell } from "lucide-react";
import { cn } from "@/lib/utils";
import { useUnreadNotifications } from "@/hooks/useUnreadNotifications";
import { useActiveAnnouncements } from "@/hooks/useActiveAnnouncements";
import { NotificationPanel } from "@/components/NotificationPanel";
import { primaryNavItems, profileNavItem } from "@/components/layout/navItems";

const tabItemClass =
  "flex flex-col items-center justify-center gap-0.5 py-2 text-[10px] font-medium transition-colors";

// Primary in-app navigation, thumb-reachable on mobile — replaces the old
// hamburger-menu pattern, which hides nav behind an extra tap. Notifications
// is a popover trigger rather than a Link (see NotificationPanel) so it's
// rendered separately from the other tabs' generic map.
export function BottomTabBar({ isAdmin }: { isAdmin: boolean }) {
  const pathname = usePathname();
  const { unreadCount, refetch } = useUnreadNotifications();
  const { announcement, dismiss } = useActiveAnnouncements();
  const badgeCount = unreadCount + (announcement ? 1 : 0);

  const tabs = primaryNavItems.filter((item) => item.mobileTab !== false);
  const profile = profileNavItem(isAdmin);
  const notificationsActive = pathname === "/notifications";

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
        })}

        <NotificationPanel
          side="top"
          align="center"
          hasUnread={unreadCount > 0}
          onNotificationsRead={refetch}
          announcement={announcement}
          onDismissAnnouncement={dismiss}
          trigger={
            <button
              className={cn(
                tabItemClass,
                notificationsActive
                  ? "text-primary"
                  : "text-muted-foreground hover:text-foreground",
              )}
              aria-label={`Notifications${badgeCount > 0 ? ` (${badgeCount} unread)` : ""}`}
            >
              <span className="relative">
                <Bell className="size-5" />
                {badgeCount > 0 && (
                  <span className="absolute -top-1 -right-2 flex size-3.5 items-center justify-center rounded-full bg-destructive text-[8px] font-bold text-white leading-none">
                    {badgeCount > 9 ? "9+" : badgeCount}
                  </span>
                )}
              </span>
              Alerts
            </button>
          }
        />

        <Link
          href={profile.href}
          className={cn(
            tabItemClass,
            profile.isActive(pathname)
              ? "text-primary"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          <profile.icon className="size-5" />
          {profile.shortLabel ?? profile.label}
        </Link>
      </div>
    </nav>
  );
}
