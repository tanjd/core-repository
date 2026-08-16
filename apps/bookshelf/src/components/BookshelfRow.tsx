"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { api } from "@/lib/api";
import type { Book } from "@/lib/types";
import { Badge } from "@/components/ui/badge";

function BookSpine({ book }: { book: Book }) {
  return (
    <Link
      href={`/catalog/${book.id}`}
      className="group flex-shrink-0 w-24 md:w-28 flex flex-col focus:outline-none"
      title={book.title}
    >
      {/* Book cover — lifts on hover like a book pulled off a shelf */}
      <div className="relative aspect-[2/3] w-full rounded-t-sm overflow-hidden shadow-[2px_4px_8px_rgba(0,0,0,0.35)] group-hover:-translate-y-2 group-hover:shadow-[4px_8px_16px_rgba(0,0,0,0.4)] transition-all duration-200 ease-out">
        {book.cover_url ? (
          <Image
            src={book.cover_url}
            alt={`Cover of ${book.title}`}
            fill
            className="object-cover"
            sizes="112px"
          />
        ) : (
          <div className="flex h-full items-center justify-center bg-gradient-to-br from-slate-200 to-slate-300 dark:from-slate-700 dark:to-slate-800">
            <span className="text-[10px] text-center text-muted-foreground px-1 leading-tight line-clamp-4">
              {book.title}
            </span>
          </div>
        )}
        {/* Spine highlight (simulates book edge) */}
        <div className="absolute inset-y-0 left-0 w-1.5 bg-black/10 pointer-events-none" />
      </div>

      {/* Metadata below shelf plank */}
      <div className="pt-2 space-y-0.5">
        <p className="text-xs font-medium line-clamp-2 leading-tight">
          {book.title}
        </p>
        <p className="text-[10px] text-muted-foreground line-clamp-1">
          {book.author}
        </p>
        {typeof book.available_copies === "number" && (
          <Badge
            variant={book.available_copies > 0 ? "success" : "secondary"}
            className="text-[9px] px-1 py-0 h-4"
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
}

export function BookshelfRow({ limit = 16 }: BookshelfRowProps) {
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
        <Link
          href="/catalog"
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Browse all →
        </Link>
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
                  <div key={book.id} className="snap-start">
                    <BookSpine book={book} />
                  </div>
                ))}
          </div>
        </div>

        {/* Wooden shelf plank */}
        <div className="mt-1 h-4 rounded-sm bg-gradient-to-b from-amber-700 via-amber-800 to-amber-950 shadow-[0_4px_8px_rgba(0,0,0,0.4)]">
          {/* Wood grain texture overlay */}
          <div className="h-full w-full rounded-sm opacity-20 bg-[repeating-linear-gradient(90deg,transparent,transparent_40px,rgba(0,0,0,0.15)_40px,rgba(0,0,0,0.15)_41px)]" />
        </div>
        {/* Shelf shadow on wall below */}
        <div className="h-2 bg-gradient-to-b from-black/10 to-transparent rounded-b-sm" />
      </div>
    </section>
  );
}
