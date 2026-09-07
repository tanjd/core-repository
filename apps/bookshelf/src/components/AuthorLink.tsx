"use client";

import type { MouseEvent } from "react";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";

interface AuthorLinkProps {
  author: string;
  className?: string;
}

// Renders as a <button>, not a Next <Link>, because every caller today
// (BookCard, BookSpine) already wraps the whole card in an outer <Link> to
// the book detail page — nesting a second <a> inside it is invalid HTML and
// breaks click targeting. This mirrors RecommendButton's existing
// stopPropagation pattern for the same reason. See
// apps/bookshelf/docs/author-view-spec.md's "Entry points".
export function AuthorLink({ author, className }: AuthorLinkProps) {
  const router = useRouter();

  function handleClick(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    router.push(`/authors/${encodeURIComponent(author)}`);
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      className={cn(
        "text-left hover:text-foreground hover:underline",
        className,
      )}
    >
      {author}
    </button>
  );
}
