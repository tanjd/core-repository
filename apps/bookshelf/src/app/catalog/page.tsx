"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { Search, SlidersHorizontal, Plus } from "lucide-react";
import { api } from "@/lib/api";
import type { Book, PaginatedResult } from "@/lib/types";
import { BookCard } from "@/components/BookCard";
import { BookshelfRow } from "@/components/BookshelfRow";
import { Pagination } from "@/components/ui/Pagination";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { useOwnedBookIds } from "@/hooks/useOwnedBookIds";

const PAGE_SIZE = 20;

export default function CatalogPage() {
  const ownedBookIds = useOwnedBookIds();
  const [result, setResult] = useState<PaginatedResult<Book> | null>(null);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState("title");
  const [availableOnly, setAvailableOnly] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  async function fetchBooks(q: string, s: string, avail: boolean, p: number) {
    setLoading(true);
    setError("");
    try {
      const data = await api.getBooks({
        q: q.trim() || undefined,
        sort: s,
        available_only: avail || undefined,
        page: p,
        page_size: PAGE_SIZE,
      });
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load books");
    } finally {
      setLoading(false);
    }
  }

  // Initial load
  useEffect(() => {
    fetchBooks("", "title", false, 1);
  }, []);

  // Debounced search — reset to page 1
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setPage(1);
      fetchBooks(search, sort, availableOnly, 1);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [search, sort, availableOnly]);

  function handlePageChange(p: number) {
    setPage(p);
    fetchBooks(search, sort, availableOnly, p);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  const books = result?.items ?? [];
  const totalPages = result?.total_pages ?? 1;
  const total = result?.total ?? 0;

  return (
    <div className="flex flex-col gap-8">
      {/* Recently added bookshelf (only when not searching) */}
      {!search && <BookshelfRow limit={16} ownedBookIds={ownedBookIds} />}

      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold">Book Catalog</h1>
        <p className="text-muted-foreground text-sm">
          Browse books shared by the community
        </p>
      </div>

      {/* Search + filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <div className="relative flex-1 max-w-xl">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input
            type="search"
            placeholder="Search by title, author…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-10"
          />
        </div>

        <div className="flex items-center gap-3 flex-wrap">
          <div className="flex items-center gap-1.5">
            <SlidersHorizontal className="size-4 text-muted-foreground" />
            <Select value={sort} onValueChange={setSort}>
              <SelectTrigger className="h-10 w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="title">Title A–Z</SelectItem>
                <SelectItem value="author">Author A–Z</SelectItem>
                <SelectItem value="newest">Newest First</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center gap-2">
            <Switch
              id="available-only"
              checked={availableOnly}
              onCheckedChange={setAvailableOnly}
            />
            <Label
              htmlFor="available-only"
              className="text-sm cursor-pointer select-none"
            >
              Available only
            </Label>
          </div>
        </div>
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
      ) : books.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <p className="text-muted-foreground">No books found.</p>
          {(search || availableOnly) && (
            <button
              onClick={() => {
                setSearch("");
                setAvailableOnly(false);
              }}
              className="text-sm text-primary hover:underline"
            >
              Clear filters
            </button>
          )}
          {search.trim() && (
            <Link
              href={`/share?q=${encodeURIComponent(search.trim())}`}
              className="text-sm text-primary hover:underline"
            >
              Can&apos;t find it? Share &ldquo;{search.trim()}&rdquo; →
            </Link>
          )}
        </div>
      ) : (
        <>
          <div>
            {total > 0 && (
              <p className="text-xs text-muted-foreground mb-4">
                {total} {total === 1 ? "book" : "books"} found
              </p>
            )}
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
              {books.map((book) => (
                <BookCard
                  key={book.id}
                  book={book}
                  ownedByMe={ownedBookIds.has(book.id)}
                />
              ))}
            </div>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={handlePageChange}
          />
        </>
      )}

      {/* Mobile-only FAB — desktop already has "Share a Book" in the top
          nav; "Share" isn't a bottom-tab slot (see navItems.ts), so this is
          how mobile users reach it from the primary browse screen. Sits
          above the bottom tab bar + its safe-area inset. */}
      <Link
        href="/share"
        aria-label="Share a book"
        className="md:hidden fixed right-4 z-40 flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg active:scale-95 transition-transform"
        style={{ bottom: "calc(env(safe-area-inset-bottom) + 4.5rem)" }}
      >
        <Plus className="size-6" />
      </Link>
    </div>
  );
}
