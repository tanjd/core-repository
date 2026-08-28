import { cn } from "@/lib/utils";

interface InitialsAvatarProps {
  name: string;
  className?: string;
}

// Small circle-with-initials avatar — originally CopyCard's inline
// OwnerAvatar, extracted so the book-recommendations facepile
// (RecommendedBy) can reuse the same look rather than inventing a new one.
// See apps/bookshelf/docs/book-recommendations-spec.md's "Open questions —
// resolved" § Facepile avatar treatment.
export function InitialsAvatar({ name, className }: InitialsAvatarProps) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase() ?? "")
    .join("");
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground",
        className,
      )}
    >
      {initials || "?"}
    </span>
  );
}
