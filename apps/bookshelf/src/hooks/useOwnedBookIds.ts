"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

// Book IDs the current user owns at least one copy of — used to surface a
// "Yours" badge in catalog views.
export function useOwnedBookIds() {
  const [ownedBookIds, setOwnedBookIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) return;
    api
      .getMyOwnedBookIds()
      .then(({ book_ids }) => {
        setOwnedBookIds(new Set(book_ids));
      })
      .catch(() => {
        // silently ignore — user may not be logged in, or request failed
      });
  }, []);

  return ownedBookIds;
}
