"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell, CheckCheck } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Notification, PaginatedResult } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/Pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const typeLabel: Record<Notification["type"], string> = {
  request_received: "New borrow request",
  request_accepted: "Request accepted",
  request_rejected: "Request declined",
  marked_loaned: "Book loaned out",
  marked_returned: "Book returned",
  waitlist_available: "Copy now available",
  copy_transferred_in: "Copy transferred to you",
  copy_transferred_out: "Copy transfer sent",
};

const PAGE_SIZE = 20;

export default function NotificationsPage() {
  const router = useRouter();
  const [result, setResult] = useState<PaginatedResult<Notification> | null>(
    null,
  );
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [markingAll, setMarkingAll] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) {
      router.push("/login");
      return;
    }
    loadNotifications(1);
  }, [router]);

  async function loadNotifications(p: number) {
    setLoading(true);
    try {
      const data = await api.getNotifications({
        page: p,
        page_size: PAGE_SIZE,
      });
      setResult(data);
      setPage(p);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to load notifications",
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleMarkRead(notification: Notification) {
    if (!notification.read) {
      try {
        await api.markNotificationRead(notification.id);
        setResult((prev) =>
          prev
            ? {
                ...prev,
                items: prev.items.map((n) =>
                  n.id === notification.id ? { ...n, read: true } : n,
                ),
              }
            : prev,
        );
      } catch {
        // silently ignore
      }
    }
    if (notification.loan_request_id) {
      if (notification.type === "request_received") {
        try {
          const lr = await api.getLoanRequest(notification.loan_request_id);
          router.push(`/my-books/${lr.copy_id}/requests`);
        } catch {
          router.push("/my-books");
        }
      } else if (notification.type === "waitlist_available") {
        router.push("/catalog");
      } else if (notification.type === "copy_transferred_in") {
        router.push("/my-books");
      } else {
        router.push("/my-requests");
      }
    }
  }

  async function handleMarkAllRead() {
    setMarkingAll(true);
    try {
      await api.markAllRead();
      setResult((prev) =>
        prev
          ? {
              ...prev,
              items: prev.items.map((n) => ({ ...n, read: true })),
            }
          : prev,
      );
      toast.success("All notifications marked as read");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to mark all read",
      );
    } finally {
      setMarkingAll(false);
    }
  }

  const notifications = result?.items ?? [];
  const unreadCount = notifications.filter((n) => !n.read).length;

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-8 w-48" />
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-16 rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6 max-w-2xl mx-auto">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h1 className="text-2xl font-bold">Notifications</h1>
          {unreadCount > 0 && (
            <Badge variant="destructive">{unreadCount} unread</Badge>
          )}
        </div>
        {unreadCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={handleMarkAllRead}
            disabled={markingAll}
          >
            <CheckCheck className="size-4" />
            {markingAll ? "Marking…" : "Mark all read"}
          </Button>
        )}
      </div>

      {notifications.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <Bell className="size-10 text-muted-foreground/40" />
          <p className="text-muted-foreground">No notifications yet.</p>
        </div>
      ) : (
        <>
          <div className="flex flex-col gap-2">
            {notifications.map((n) => (
              <button
                key={n.id}
                onClick={() => handleMarkRead(n)}
                className={cn(
                  "flex items-start gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent",
                  !n.read && "bg-muted/60 border-primary/20",
                )}
              >
                <div className="mt-0.5 shrink-0">
                  {!n.read ? (
                    <span className="block size-2 rounded-full bg-primary" />
                  ) : (
                    <span className="block size-2 rounded-full bg-muted-foreground/30" />
                  )}
                </div>
                <div className="flex flex-col gap-0.5 min-w-0">
                  <p className={cn("text-sm", !n.read && "font-medium")}>
                    {typeLabel[n.type] ?? n.type}
                  </p>
                  {n.type === "waitlist_available" && (
                    <p className="text-xs text-muted-foreground">
                      A copy you waitlisted is now available — go request it!
                    </p>
                  )}
                  {n.loan_request_id && n.type !== "waitlist_available" && (
                    <p className="text-xs text-muted-foreground">
                      Request #{n.loan_request_id}
                    </p>
                  )}
                  <p className="text-xs text-muted-foreground">
                    {new Date(n.created_at).toLocaleString()}
                  </p>
                </div>
                {!n.read && (
                  <Badge
                    variant="secondary"
                    className="ml-auto shrink-0 text-[10px]"
                  >
                    New
                  </Badge>
                )}
              </button>
            ))}
          </div>
          <Pagination
            page={page}
            totalPages={result?.total_pages ?? 1}
            onPageChange={loadNotifications}
          />
        </>
      )}
    </div>
  );
}
