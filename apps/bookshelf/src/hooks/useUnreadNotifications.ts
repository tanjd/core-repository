"use client";

import { useCallback, useEffect, useState, useRef } from "react";
import { api } from "@/lib/api";

const POLL_INTERVAL_MS = 60_000;

export function useUnreadNotifications() {
  const [unreadCount, setUnreadCount] = useState(0);

  const fetchUnread = useCallback(async () => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) return;
    try {
      const result = await api.getNotifications({ unread: true, page_size: 1 });
      setUnreadCount(result.total);
    } catch {
      // silently ignore — user may not be logged in
    }
  }, []);

  const startedRef = useRef(false);
  useEffect(() => {
    if (!startedRef.current) {
      startedRef.current = true;
      fetchUnread();
    }
    const interval = setInterval(fetchUnread, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [fetchUnread]);

  return { unreadCount, refetch: fetchUnread };
}
