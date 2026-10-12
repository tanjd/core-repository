"use client";

import { useState, type MouseEvent } from "react";
import { Bookmark } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";

interface ToReadButtonProps {
  bookId: number;
  bookTitle: string;
  // Seed value from the already-loaded Book (your_to_read, batch-populated
  // by the backend) — same pattern as RecommendButton; the button then owns
  // its own state after mount rather than re-syncing from a parent
  // re-render.
  saved: boolean;
  className?: string;
  onToggled?: () => void;
}

// Icon-only "save to read later" toggle — a personal bookmark on a catalog
// book, shared by BookCard and the book detail page so tapping either
// surface acts on the same underlying save. Unlike RecommendButton, this
// has no visible count — it's a private list, not a community signal.
export function ToReadButton({
  bookId,
  bookTitle,
  saved: initialSaved,
  className,
  onToggled,
}: ToReadButtonProps) {
  const [saved, setSaved] = useState(initialSaved);
  const [pending, setPending] = useState(false);

  async function toggle(e: MouseEvent) {
    // BookCard wraps this in a <Link> to the detail page — without this,
    // tapping the button would also navigate away.
    e.preventDefault();
    e.stopPropagation();
    if (pending) return;

    const prevSaved = saved;
    const nextSaved = !saved;

    // Optimistic: flip immediately, before the server confirms. Reverted
    // below on failure.
    setPending(true);
    setSaved(nextSaved);
    try {
      if (nextSaved) {
        await api.addToRead(bookId);
      } else {
        await api.removeToRead(bookId);
      }
      onToggled?.();
    } catch (err) {
      setSaved(prevSaved);
      toast.error(
        err instanceof Error
          ? err.message
          : "Failed to update your to-read list",
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
      aria-pressed={saved}
      aria-label={
        saved
          ? `Remove ${bookTitle} from your to-read list`
          : `Save ${bookTitle} to read later`
      }
      title={
        saved
          ? "On your to-read list — click to remove"
          : "Save this book to read later"
      }
      className={cn(
        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs transition-colors disabled:opacity-60",
        saved
          ? "bg-primary/10 text-primary"
          : "text-muted-foreground hover:bg-muted hover:text-foreground",
        className,
      )}
    >
      <Bookmark
        className={cn("size-3.5", saved && "fill-current")}
        aria-hidden="true"
      />
    </button>
  );
}
