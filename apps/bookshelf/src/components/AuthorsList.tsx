"use client";

import Link from "next/link";
import type { AuthorSummary } from "@/lib/types";
import { BookCover } from "@/components/BookCover";

// A small overlapping stack of up to 3 covers, giving each author row the
// same cover-driven visual anchor as every other book-listing surface in
// this app (BookCard, BookshelfRow) instead of reading as plain text.
// Authors with no sampled covers (none of their books have a cover_url)
// still get one slot, rendered via BookCover's built-in BookCoverFallback —
// same as every other surface's missing-cover handling.
function AuthorCoverStack({ author, covers = [] }: AuthorSummary) {
  const displayCovers = covers.length > 0 ? covers : [undefined];
  return (
    <div className="flex shrink-0">
      {displayCovers.map((coverUrl, i) => (
        <div
          key={i}
          className="relative aspect-[2/3] w-8 overflow-hidden rounded-sm border border-background bg-muted shadow-sm"
          style={i > 0 ? { marginLeft: "-0.75rem" } : undefined}
        >
          <BookCover title={author} coverUrl={coverUrl} sizes="32px" />
        </div>
      ))}
    </div>
  );
}

// Alphabetical, sticky-letter-header list of distinct authors — the
// "Authors" view nested inside the Catalog page. Grouping is exact-string
// only (same rule as the per-author page itself); see
// apps/bookshelf/docs/author-view-spec.md's "Authors index page". Grouping
// happens client-side over whatever page of already-alphabetized authors
// the backend returned — a page never spans more than the current
// PAGE_SIZE, so this is just clustering consecutive same-letter rows, not
// a full re-sort.
export function AuthorsList({
  authors,
  query,
  fromHref,
}: {
  authors: AuthorSummary[];
  // The active author-search filter, if any — used only to word the empty
  // state; filtering itself already happened server-side.
  query?: string;
  // The current Authors-tab URL (view=authors, plus authorQ/page), baked
  // into each author link as ?from= so the per-author page's "Catalog"
  // breadcrumb can restore this exact view instead of the Books tab.
  fromHref?: string;
}) {
  if (authors.length === 0) {
    return (
      <p className="text-muted-foreground py-8 text-center">
        {query ? `No authors found matching “${query}”.` : "No authors found."}
      </p>
    );
  }

  const groups: { letter: string; authors: AuthorSummary[] }[] = [];
  for (const author of authors) {
    if (!author.author) continue;
    const letter = author.author.charAt(0).toUpperCase() || "#";
    const lastGroup = groups[groups.length - 1];
    if (lastGroup?.letter === letter) {
      lastGroup.authors.push(author);
    } else {
      groups.push({ letter, authors: [author] });
    }
  }

  return (
    <div className="flex flex-col">
      {groups.map((group) => (
        <div key={group.letter}>
          <div className="sticky top-0 z-10 bg-background/95 backdrop-blur-sm border-b px-1 py-1.5 text-xs font-semibold text-muted-foreground">
            {group.letter}
          </div>
          {group.authors.map((author) => (
            <Link
              key={author.author}
              href={
                fromHref
                  ? `/authors/${encodeURIComponent(author.author)}?from=${encodeURIComponent(fromHref)}`
                  : `/authors/${encodeURIComponent(author.author)}`
              }
              className="flex items-center justify-between gap-3 px-1 py-3 border-b hover:bg-accent/50"
            >
              <span className="flex items-center gap-3 min-w-0">
                <AuthorCoverStack {...author} />
                <span className="font-medium truncate">{author.author}</span>
              </span>
              <span className="text-sm text-muted-foreground shrink-0">
                {author.book_count} {author.book_count === 1 ? "book" : "books"}
              </span>
            </Link>
          ))}
        </div>
      ))}
    </div>
  );
}
