"use client";

import { useRouter } from "next/navigation";
import { Bell } from "lucide-react";
import { useUnreadNotifications } from "@/hooks/useUnreadNotifications";

export function NotificationBell() {
  const router = useRouter();
  const unreadCount = useUnreadNotifications();

  return (
    <button
      onClick={() => router.push("/notifications")}
      className="relative p-2 rounded-md hover:bg-accent transition-colors"
      aria-label={`Notifications${unreadCount > 0 ? ` (${unreadCount} unread)` : ""}`}
    >
      <Bell className="size-5" />
      {unreadCount > 0 && (
        <span className="absolute top-1 right-1 flex size-4 items-center justify-center rounded-full bg-destructive text-[10px] font-bold text-white leading-none">
          {unreadCount > 99 ? "99+" : unreadCount}
        </span>
      )}
    </button>
  );
}
