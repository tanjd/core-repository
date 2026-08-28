"use client";

import { useEffect, useState } from "react";
import { api, type Recommendation } from "@/lib/api";
import { InitialsAvatar } from "@/components/InitialsAvatar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

interface RecommendedByProps {
  bookId: number;
  // Bumped by the parent whenever the caller's own recommend toggle
  // resolves, so the facepile re-fetches and reflects the change without
  // the parent needing to know the list shape.
  refreshKey?: number;
}

const MAX_SHOWN = 3;

// Facepile of members who currently recommend a book — see
// apps/bookshelf/docs/book-recommendations-spec.md's "Detail-page surface".
// Renders nothing at all (not even a wrapper) when nobody has recommended
// the book yet — no "recommended by nobody" empty state.
export function RecommendedBy({ bookId, refreshKey }: RecommendedByProps) {
  const [recommenders, setRecommenders] = useState<Recommendation[] | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    api
      .getRecommendations(bookId)
      .then((list) => {
        if (!cancelled) setRecommenders(list);
      })
      .catch(() => {
        if (!cancelled) setRecommenders([]);
      });
    return () => {
      cancelled = true;
    };
  }, [bookId, refreshKey]);

  if (!recommenders || recommenders.length === 0) return null;

  const shown = recommenders.slice(0, MAX_SHOWN);
  const overflow = recommenders.length - shown.length;

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-muted-foreground">Recommended by</span>
      <div className="flex -space-x-2">
        {shown.map((r) => (
          <InitialsAvatar
            key={`${r.recommender_name}-${r.created_at}`}
            name={r.recommender_name}
            className="ring-2 ring-background"
          />
        ))}
      </div>
      {overflow > 0 && (
        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="text-xs text-muted-foreground hover:text-foreground hover:underline"
            >
              and {overflow} other{overflow === 1 ? "" : "s"}
            </button>
          </PopoverTrigger>
          <PopoverContent className="w-56">
            <ul className="flex flex-col gap-1 text-sm">
              {recommenders.map((r) => (
                <li key={`${r.recommender_name}-${r.created_at}`}>
                  {r.recommender_name}
                </li>
              ))}
            </ul>
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
