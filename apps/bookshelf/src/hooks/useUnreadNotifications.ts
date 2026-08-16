"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";

const POLL_INTERVAL_MS = 30_000;

export function useUnreadNotifications() {
  const [unreadCount, setUnreadCount] = useState(0);

  const fetchUnread = useCallback(async () => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) return;
    try {
      const result = await api.getNotifications({
        unread: true,
        page_size: 100,
      });
      setUnreadCount(result.items.filter((n) => !n.read).length);
    } catch {
      // silently ignore — user may not be logged in
    }
  }, []);

  useEffect(() => {
    fetchUnread();
    const interval = setInterval(fetchUnread, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [fetchUnread]);

  return unreadCount;
}
