"use client";

import { Bell } from "lucide-react";
import { useUnreadNotifications } from "@/hooks/useUnreadNotifications";
import { useActiveAnnouncements } from "@/hooks/useActiveAnnouncements";
import { useUpgradeNotice } from "@/hooks/useUpgradeNotice";
import { NotificationPanel } from "@/components/NotificationPanel";

export function NotificationBell() {
  const { unreadCount, refetch } = useUnreadNotifications();
  const { announcement, dismiss } = useActiveAnnouncements();
  const {
    visible: upgradeVisible,
    version,
    dismiss: dismissUpgrade,
  } = useUpgradeNotice();
  const badgeCount =
    unreadCount + (upgradeVisible ? 1 : 0) + (announcement ? 1 : 0);

  return (
    <NotificationPanel
      side="bottom"
      align="end"
      hasUnread={unreadCount > 0}
      onNotificationsRead={refetch}
      upgradeNotice={
        upgradeVisible ? { version, onDismiss: dismissUpgrade } : null
      }
      announcement={announcement}
      onDismissAnnouncement={dismiss}
      trigger={
        <button
          className="relative p-2 rounded-md hover:bg-accent transition-colors"
          aria-label={`Notifications${badgeCount > 0 ? ` (${badgeCount} unread)` : ""}`}
        >
          <Bell className="size-5" />
          {badgeCount > 0 && (
            <span className="absolute top-1 right-1 flex size-4 items-center justify-center rounded-full bg-destructive text-[10px] font-bold text-white leading-none">
              {badgeCount > 99 ? "99+" : badgeCount}
            </span>
          )}
        </button>
      }
    />
  );
}
