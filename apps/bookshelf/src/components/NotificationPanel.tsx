"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Bell, CheckCheck, Loader2, Megaphone, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Announcement, Notification } from "@/lib/types";
import {
  notificationDestination,
  notificationTypeLabel,
} from "@/lib/notifications";
import {
  announcementTypeLabel,
  announcementTypeVariant,
} from "@/lib/announcements";
import { Badge } from "@/components/ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

const PREVIEW_SIZE = 8;

export function NotificationPanel({
  trigger,
  hasUnread,
  onNotificationsRead,
  announcement = null,
  onDismissAnnouncement,
  side = "bottom",
  align = "end",
}: {
  trigger: ReactNode;
  hasUnread: boolean;
  onNotificationsRead?: () => void;
  announcement?: Announcement | null;
  onDismissAnnouncement?: (id: number) => void;
  side?: "top" | "bottom";
  align?: "start" | "center" | "end";
}) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<Notification[] | null>(null);
  const [markingAll, setMarkingAll] = useState(false);

  async function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next && items === null) {
      setLoading(true);
      try {
        const data = await api.getNotifications({ page_size: PREVIEW_SIZE });
        setItems(data.items);
      } catch {
        setItems([]);
      } finally {
        setLoading(false);
      }
    }
  }

  async function handleSelect(n: Notification) {
    if (!n.read) {
      setItems(
        (prev) =>
          prev?.map((i) => (i.id === n.id ? { ...i, read: true } : i)) ?? prev,
      );
      api
        .markNotificationRead(n.id)
        .then(() => onNotificationsRead?.())
        .catch(() => undefined);
    }
    const dest = await notificationDestination(n);
    setOpen(false);
    if (dest) router.push(dest);
  }

  async function handleMarkAllRead() {
    setMarkingAll(true);
    try {
      await api.markAllRead();
      setItems((prev) => prev?.map((n) => ({ ...n, read: true })) ?? prev);
      onNotificationsRead?.();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to mark all read",
      );
    } finally {
      setMarkingAll(false);
    }
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent
        side={side}
        align={align}
        className="w-[calc(100vw-2rem)] max-w-sm p-0 sm:w-96"
      >
        {announcement && (
          <div className="border-b">
            <div className="flex items-center gap-1.5 px-4 pt-3 pb-1.5">
              <Megaphone className="size-3.5 text-muted-foreground" />
              <span className="text-xs font-semibold text-muted-foreground">
                Announcement
              </span>
            </div>
            <div className="flex items-start gap-2 px-4 pb-3">
              <div className="min-w-0 flex-1">
                <div className="mb-0.5 flex items-center gap-1.5">
                  <Badge variant={announcementTypeVariant[announcement.type]}>
                    {announcementTypeLabel[announcement.type]}
                  </Badge>
                  <span className="truncate text-sm font-medium">
                    {announcement.title}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground">
                  {announcement.body}
                </p>
              </div>
              {onDismissAnnouncement && (
                <button
                  onClick={() => onDismissAnnouncement(announcement.id)}
                  className="shrink-0 rounded p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                  aria-label="Dismiss announcement"
                >
                  <X className="size-3.5" />
                </button>
              )}
            </div>
          </div>
        )}

        <div className="flex items-center justify-between border-b px-4 py-3">
          <span className="text-sm font-semibold">Notifications</span>
          {hasUnread && (
            <button
              onClick={handleMarkAllRead}
              disabled={markingAll}
              className="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
            >
              <CheckCheck className="size-3.5" />
              {markingAll ? "Marking…" : "Mark all read"}
            </button>
          )}
        </div>

        <div className="max-h-[60vh] overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-10">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : !items || items.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-10 text-center">
              <Bell className="size-8 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">
                No notifications yet.
              </p>
            </div>
          ) : (
            <div className="flex flex-col divide-y">
              {items.map((n) => (
                <button
                  key={n.id}
                  onClick={() => handleSelect(n)}
                  className={cn(
                    "flex items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-accent",
                    !n.read && "bg-muted/60",
                  )}
                >
                  <span
                    className={cn(
                      "mt-1.5 block size-2 shrink-0 rounded-full",
                      n.read ? "bg-muted-foreground/30" : "bg-primary",
                    )}
                  />
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <p className={cn("text-sm", !n.read && "font-medium")}>
                      {notificationTypeLabel[n.type] ?? n.type}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {new Date(n.created_at).toLocaleString()}
                    </p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        <Link
          href="/notifications"
          onClick={() => setOpen(false)}
          className="block border-t px-4 py-2.5 text-center text-sm font-medium text-primary transition-colors hover:bg-accent"
        >
          See all notifications
        </Link>
      </PopoverContent>
    </Popover>
  );
}
