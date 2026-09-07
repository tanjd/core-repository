"use client";

import { useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { Book, PaginatedResult } from "@/lib/types";
import { BookCard } from "@/components/BookCard";
import { Breadcrumb } from "@/components/Breadcrumb";
import { Pagination } from "@/components/ui/Pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useOwnedBookIds } from "@/hooks/useOwnedBookIds";

const PAGE_SIZE = 20;

// Every book by one exact author string, reusing the same BookCard grid and
// pagination the Catalog page uses — see
// apps/bookshelf/docs/author-view-spec.md. No empty state: this page is only
// ever reached by tapping an author name on a book that has it, or a row in
// the Authors index that's already counted at least one book.
export default function AuthorPage() {
  const params = useParams<{ author: string }>();
  const author = decodeURIComponent(params.author);
  const ownedBookIds = useOwnedBookIds();
  const [result, setResult] = useState<PaginatedResult<Book> | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const requestIdRef = useRef(0);

  async function fetchBooks(p: number) {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setError("");
    try {
      const data = await api.getBooksByAuthor({
        author,
        page: p,
        page_size: PAGE_SIZE,
      });
      if (requestId !== requestIdRef.current) return;
      setResult(data);
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err.message : "Failed to load books");
    } finally {
      if (requestId === requestIdRef.current) setLoading(false);
    }
  }

  useEffect(() => {
    // Refetches on every author change (App Router doesn't remount this
    // page across sibling dynamic-segment navigations) — same
    // setState-in-effect exception as CatalogPage's URL hydration on mount.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void fetchBooks(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [author]);

  function handlePageChange(p: number) {
    setPage(p);
    fetchBooks(p);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  const books = result?.items ?? [];
  const totalPages = result?.total_pages ?? 1;
  const total = result?.total ?? 0;

  return (
    <div className="flex flex-col gap-6">
      <Breadcrumb
        back={{ href: "/catalog" }}
        backLabel="Catalog"
        current={author}
      />

      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold">{author}</h1>
        {!loading && (
          <p className="text-muted-foreground text-sm">
            {total} {total === 1 ? "book" : "books"} in the catalog
          </p>
        )}
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
          {Array.from({ length: 10 }).map((_, i) => (
            <div key={i}>
              <Skeleton className="aspect-[2/3] rounded-lg" />
              <Skeleton className="mt-2 h-4 w-3/4" />
              <Skeleton className="mt-1 h-3 w-1/2" />
            </div>
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
            {books.map((book) => (
              <BookCard
                key={book.id}
                book={book}
                ownedByMe={ownedBookIds.has(book.id)}
                catalogHref={`/authors/${encodeURIComponent(author)}`}
              />
            ))}
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={handlePageChange}
          />
        </>
      )}
    </div>
  );
}
