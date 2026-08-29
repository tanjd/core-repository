"use client";

import { useState, type MouseEvent } from "react";
import { ThumbsUp } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

interface RecommendButtonProps {
  bookId: number;
  bookTitle: string;
  // Seed values from the already-loaded Book (recommendation_count /
  // your_recommendation, batch-populated by the backend) — same pattern as
  // every other per-card affordance in this app; the button then owns its
  // own state after mount rather than re-syncing from a parent re-render,
  // matching WaitlistButton's convention.
  recommended: boolean;
  count: number;
  className?: string;
  // Fired after a successful add/remove (not on the optimistic flip, and
  // not on failure/rollback) — lets a parent that also shows a facepile
  // (RecommendedBy) know to re-fetch so the viewer's own entry appears or
  // disappears from it too.
  onToggled?: () => void;
}

// Icon-only "highly recommend this" toggle — shared by BookCard and the
// book detail page so tapping either surface acts on the same underlying
// thumbs-up. See apps/bookshelf/docs/book-recommendations-spec.md's
// "Catalog surface" / "Detail-page surface" / "Failure UX" / "Accessibility"
// sections.
export function RecommendButton({
  bookId,
  bookTitle,
  recommended: initialRecommended,
  count: initialCount,
  className,
  onToggled,
}: RecommendButtonProps) {
  const [recommended, setRecommended] = useState(initialRecommended);
  const [count, setCount] = useState(initialCount);
  const [pending, setPending] = useState(false);

  async function toggle(e: MouseEvent) {
    // BookCard wraps this in a <Link> to the detail page — without this,
    // tapping the button would also navigate away.
    e.preventDefault();
    e.stopPropagation();
    if (pending) return;

    const prevRecommended = recommended;
    const prevCount = count;
    const nextRecommended = !recommended;
    const nextCount = nextRecommended ? count + 1 : Math.max(0, count - 1);

    // Optimistic: flip immediately, before the server confirms — see the
    // spec's "Failure UX". Reverted below on failure.
    setPending(true);
    setRecommended(nextRecommended);
    setCount(nextCount);
    try {
      if (nextRecommended) {
        await api.recommendBook(bookId);
      } else {
        await api.unrecommendBook(bookId);
      }
      onToggled?.();
    } catch (err) {
      setRecommended(prevRecommended);
      setCount(prevCount);
      toast.error(
        err instanceof Error
          ? err.message
          : "Failed to update your recommendation",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      disabled={pending}
      aria-pressed={recommended}
      aria-label={
        recommended
          ? `Remove your recommendation for ${bookTitle}`
          : `Recommend ${bookTitle}`
      }
      title={
        recommended
          ? "You recommended this book — click to remove"
          : "Thumbs up to recommend this book to other members"
      }
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs transition-colors disabled:opacity-60",
        recommended
          ? "bg-primary/10 text-primary"
          : "text-muted-foreground hover:bg-muted hover:text-foreground",
        className,
      )}
    >
      <ThumbsUp
        className={cn("size-3.5", recommended && "fill-current")}
        aria-hidden="true"
      />
      {count > 0 && <span>{count}</span>}
    </button>
  );
}
