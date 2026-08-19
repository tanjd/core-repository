"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { Announcement } from "@/lib/types";

const DISMISSED_KEY = "bookshelf_dismissed_announcement_id";

function readDismissedId(): number | null {
  try {
    const raw = localStorage.getItem(DISMISSED_KEY);
    return raw ? Number(raw) : null;
  } catch {
    return null;
  }
}

// Fetches active announcements once per mount (not polled, unlike
// useUnreadNotifications — this is a small, admin-curated list that doesn't
// need near-real-time updates) and surfaces only the single most recent
// active one (the backend already orders ListActive by created_at desc).
// Showing one at a time, rather than stacking every active announcement,
// keeps the notification panel simple — an admin wanting to say two things
// at once just folds them into one announcement's body. Dismissal is
// client-side only (localStorage), not tracked server-side — an
// announcement stays "active" for everyone until an admin deactivates it;
// dismissing just stops showing it to this browser.
export function useActiveAnnouncements() {
  const [announcement, setAnnouncement] = useState<Announcement | null>(null);
  const [dismissedId, setDismissedId] = useState<number | null>(null);

  const fetchAnnouncements = useCallback(async () => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) return;
    try {
      const items = await api.getActiveAnnouncements();
      setAnnouncement(items[0] ?? null);
    } catch {
      // silently ignore — user may not be logged in, or backend unreachable
    }
  }, []);

  const startedRef = useRef(false);
  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;
    setDismissedId(readDismissedId());
    fetchAnnouncements();
  }, [fetchAnnouncements]);

  const dismiss = useCallback((id: number) => {
    setDismissedId(id);
    localStorage.setItem(DISMISSED_KEY, String(id));
  }, []);

  return {
    announcement:
      announcement && announcement.id === dismissedId ? null : announcement,
    dismiss,
  };
}
