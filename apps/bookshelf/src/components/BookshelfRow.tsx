"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Book } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { BookCover } from "@/components/BookCover";

function BookSpine({
  book,
  ownedByMe,
  catalogHref,
}: {
  book: Book;
  ownedByMe: boolean;
  catalogHref?: string;
}) {
  const detailHref = catalogHref
    ? `/catalog/${book.id}?from=${encodeURIComponent(catalogHref)}`
    : `/catalog/${book.id}`;
  return (
    <Link
      href={detailHref}
      className="group flex-shrink-0 w-24 md:w-28 flex flex-col focus:outline-none"
      title={book.title}
    >
      {/* Book cover — lifts on hover like a book pulled off a shelf */}
      <div className="relative aspect-[2/3] w-full rounded-t-sm overflow-hidden shadow-[2px_4px_8px_rgba(0,0,0,0.35)] group-hover:-translate-y-2 group-hover:shadow-[4px_8px_16px_rgba(0,0,0,0.4)] transition-all duration-200 ease-out">
        {ownedByMe && (
          <Badge className="absolute top-1.5 left-1.5 z-10 shadow-sm text-[9px] px-1.5 py-0.5 leading-none">
            Yours
          </Badge>
        )}
        <BookCover
          title={book.title}
          author={book.author}
          coverUrl={book.cover_url}
          sizes="112px"
        />
        {/* Spine highlight (simulates book edge) */}
        <div className="absolute inset-y-0 left-0 w-1.5 bg-black/10 pointer-events-none" />
      </div>

      {/* Metadata below shelf plank */}
      <div className="pt-2 flex flex-1 flex-col space-y-0.5">
        <p className="text-xs font-medium line-clamp-2 leading-tight">
          {book.title}
        </p>
        <p className="text-[10px] text-muted-foreground line-clamp-1">
          {book.author}
        </p>
        {typeof book.available_copies === "number" && (
          <Badge
            variant={book.available_copies > 0 ? "success" : "secondary"}
            className="text-[9px] px-1.5 py-0.5 leading-none mt-auto self-start"
          >
            {book.available_copies > 0
              ? `${book.available_copies} avail.`
              : "Out"}
          </Badge>
        )}
      </div>
    </Link>
  );
}

function BookSpineSkeleton() {
  return (
    <div className="flex-shrink-0 w-24 md:w-28 flex flex-col gap-2 animate-pulse">
      <div className="aspect-[2/3] w-full rounded-t-sm bg-muted" />
      <div className="h-3 rounded bg-muted w-4/5" />
      <div className="h-2.5 rounded bg-muted w-3/5" />
    </div>
  );
}

interface BookshelfRowProps {
  limit?: number;
  ownedBookIds: Set<number>;
  catalogHref?: string;
}

export function BookshelfRow({
  limit = 12,
  ownedBookIds,
  catalogHref,
}: BookshelfRowProps) {
  const [books, setBooks] = useState<Book[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .getRecentBooks(limit)
      .then(setBooks)
      .catch(() => setBooks([]))
      .finally(() => setLoading(false));
  }, [limit]);

  if (!loading && books.length === 0) return null;

  return (
    <section className="w-full">
      {/* Section heading */}
      <div className="flex items-baseline justify-between mb-3">
        <h2 className="text-xl font-bold tracking-tight">Recently Added</h2>
      </div>

      {/* Shelf container */}
      <div className="relative">
        {/* Books row — scrollable */}
        <div className="overflow-x-auto [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
          <div className="flex gap-3 px-1 pb-0 snap-x snap-mandatory min-w-max">
            {loading
              ? Array.from({ length: 8 }).map((_, i) => (
                  <BookSpineSkeleton key={i} />
                ))
              : books.map((book) => (
                  <div key={book.id} className="snap-start flex">
                    <BookSpine
                      book={book}
                      ownedByMe={ownedBookIds.has(book.id)}
                      catalogHref={catalogHref}
                    />
                  </div>
                ))}
          </div>
        </div>

        {/* Shelf line — flat, token-driven divider instead of a literal
            wood-grain plank, so it doesn't clash with the rest of the app's
            neutral palette; the cover-lift-on-hover effect above still
            carries the "pulling a book off a shelf" feeling on its own. */}
        <div className="mt-1 h-px bg-border" />
        <div className="h-2 bg-gradient-to-b from-foreground/5 to-transparent" />
      </div>
    </section>
  );
}
