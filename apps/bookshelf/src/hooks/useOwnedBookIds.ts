"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

// Book IDs the current user owns at least one copy of — used to surface a
// "Yours" badge in catalog views without needing a backend change.
export function useOwnedBookIds() {
  const [ownedBookIds, setOwnedBookIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) return;
    api
      .getMyCopies()
      .then((copies) => {
        setOwnedBookIds(new Set(copies.map((c) => c.book_id)));
      })
      .catch(() => {
        // silently ignore — user may not be logged in, or request failed
      });
  }, []);

  return ownedBookIds;
}
