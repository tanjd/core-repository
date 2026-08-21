"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import {
  Search,
  SlidersHorizontal,
  Plus,
  PlusCircle,
  ScanLine,
  Heart,
  X,
  Loader2,
} from "lucide-react";
import { api } from "@/lib/api";
import type { Book, PaginatedResult } from "@/lib/types";
import { BookCard } from "@/components/BookCard";
import { BookshelfRow } from "@/components/BookshelfRow";
import { Pagination } from "@/components/ui/Pagination";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useOwnedBookIds } from "@/hooks/useOwnedBookIds";

const PAGE_SIZE = 20;
const SORT_LABELS: Record<string, string> = {
  title: "Title A–Z",
  author: "Author A–Z",
  newest: "Newest First",
  relevance: "Best Match",
};

export default function CatalogPage() {
  const ownedBookIds = useOwnedBookIds();
  const [result, setResult] = useState<PaginatedResult<Book> | null>(null);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState("title");
  const [availableOnly, setAvailableOnly] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [fetching, setFetching] = useState(false);
  const [error, setError] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Tracks whether the user has explicitly picked a sort — once they have,
  // typing/clearing a search no longer auto-switches it for them.
  const sortTouchedRef = useRef(false);

  async function fetchBooks(
    q: string,
    s: string,
    avail: boolean,
    p: number,
    isInitial = false,
  ) {
    if (isInitial) setLoading(true);
    else setFetching(true);
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
      if (isInitial) setLoading(false);
      else setFetching(false);
    }
  }

  // Initial load
  const loadedRef = useRef(false);
  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    fetchBooks("", "title", false, 1, true);
  }, []);

  // Debounced search/sort/filter — reset to page 1. Skips the mount pass
  // (the effect above already fetched the initial page).
  const mountedRef = useRef(false);
  useEffect(() => {
    if (!mountedRef.current) {
      mountedRef.current = true;
      return;
    }
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setPage(1);
      fetchBooks(search, sort, availableOnly, 1);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [search, sort, availableOnly]);

  function handleSearchChange(value: string) {
    setSearch(value);
    if (value.trim()) {
      if (!sortTouchedRef.current && sort !== "relevance") {
        setSort("relevance");
      }
    } else if (sort === "relevance") {
      // "Best match" is meaningless without a query — fall back, and let
      // the next search re-suggest it rather than sticking on a stale pick.
      setSort("title");
      sortTouchedRef.current = false;
    }
  }

  function handleSortChange(value: string) {
    sortTouchedRef.current = true;
    setSort(value);
  }

  function clearFilters() {
    sortTouchedRef.current = false;
    setSearch("");
    setSort("title");
    setAvailableOnly(false);
  }

  function handlePageChange(p: number) {
    setPage(p);
    fetchBooks(search, sort, availableOnly, p);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  const books = result?.items ?? [];
  const totalPages = result?.total_pages ?? 1;
  const total = result?.total ?? 0;
  const hasActiveFilters = !!search.trim() || availableOnly || sort !== "title";

  return (
    <div className="flex flex-col gap-8">
      {/* Recently added bookshelf (only when not searching) */}
      {!search && <BookshelfRow limit={12} ownedBookIds={ownedBookIds} />}

      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-bold">Book Catalog</h1>
        <p className="text-muted-foreground text-sm">
          Browse books shared by the community
        </p>
      </div>

      {/* Search + filters */}
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <div className="relative flex-1 max-w-xl">
          {fetching ? (
            <Loader2 className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none animate-spin" />
          ) : (
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          )}
          <Input
            type="search"
            placeholder="Search by title, author…"
            value={search}
            onChange={(e) => handleSearchChange(e.target.value)}
            className="pl-9 h-10 pr-9"
          />
          {search && (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => handleSearchChange("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 flex size-6 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          )}
        </div>

        <div className="flex items-center gap-3 flex-wrap">
          <div className="flex items-center gap-1.5">
            <SlidersHorizontal className="size-4 text-muted-foreground" />
            <Select value={sort} onValueChange={handleSortChange}>
              <SelectTrigger className="h-10 w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="title">Title A–Z</SelectItem>
                <SelectItem value="author">Author A–Z</SelectItem>
                <SelectItem value="newest">Newest First</SelectItem>
                {search.trim() && (
                  <SelectItem value="relevance">Best Match</SelectItem>
                )}
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

      {/* Active filter chips — only meaningful once results have loaded at
          least once; keeps the applied search/sort/availability state
          legible after the filter row above scrolls out of view. */}
      {hasActiveFilters && !loading && (
        <div className="flex items-center gap-2 flex-wrap -mt-4">
          {search.trim() && (
            <Badge variant="secondary" className="gap-1 pr-1">
              &ldquo;{search.trim()}&rdquo;
              <button
                type="button"
                aria-label="Clear search"
                onClick={() => handleSearchChange("")}
                className="rounded-full hover:bg-background/60 p-0.5"
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
          {availableOnly && (
            <Badge variant="secondary" className="gap-1 pr-1">
              Available only
              <button
                type="button"
                aria-label="Remove available-only filter"
                onClick={() => setAvailableOnly(false)}
                className="rounded-full hover:bg-background/60 p-0.5"
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
          {sort !== "title" && (
            <Badge variant="secondary" className="gap-1 pr-1">
              Sort: {SORT_LABELS[sort] ?? sort}
              <button
                type="button"
                aria-label="Reset sort"
                onClick={() => handleSortChange("title")}
                className="rounded-full hover:bg-background/60 p-0.5"
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
          <button
            onClick={clearFilters}
            className="text-xs text-muted-foreground hover:text-foreground hover:underline ml-1"
          >
            Clear all
          </button>
        </div>
      )}

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
        <div className="flex flex-col items-center justify-center py-16 text-center gap-4">
          <p className="text-muted-foreground">No books found.</p>
          <div className="flex flex-wrap items-center justify-center gap-2">
            {hasActiveFilters && (
              <Button variant="outline" size="sm" onClick={clearFilters}>
                Clear filters
              </Button>
            )}
            {search.trim() && (
              <Link href={`/share?q=${encodeURIComponent(search.trim())}`}>
                <Button variant="outline" size="sm">
                  Own a copy? Share &ldquo;{search.trim()}&rdquo;
                </Button>
              </Link>
            )}
            {search.trim() && (
              <Link href={`/wishlist?q=${encodeURIComponent(search.trim())}`}>
                <Button variant="outline" size="sm">
                  <Heart className="size-3.5" />
                  Add to wishlist
                </Button>
              </Link>
            )}
          </div>
        </div>
      ) : (
        <>
          <div
            className={
              fetching ? "opacity-60 transition-opacity" : "transition-opacity"
            }
          >
            {total > 0 && (
              <p className="text-sm text-muted-foreground mb-4">
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

      {/* Mobile-only speed-dial FAB — desktop already has "Share a Book"
          and "Wishlist" in the top nav; neither is a bottom-tab slot
          (see navItems.ts), so this is how mobile users reach either from
          the primary browse screen. "Scan ISBN" has no desktop nav
          affordance at all (most desktops lack a book-facing camera). Sits
          above the bottom tab bar + its safe-area inset. */}
      <Popover>
        <PopoverTrigger asChild>
          <button
            aria-label="Share a book or post a wishlist request"
            className="md:hidden fixed right-4 z-40 flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg active:scale-95 transition-transform data-[state=open]:rotate-45"
            style={{ bottom: "calc(env(safe-area-inset-bottom) + 4.5rem)" }}
          >
            <Plus className="size-6" />
          </button>
        </PopoverTrigger>
        <PopoverContent
          side="top"
          align="end"
          sideOffset={12}
          className="md:hidden w-52 p-1"
        >
          <Link
            href="/share"
            className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
          >
            <PlusCircle className="size-4" />
            Share a Book
          </Link>
          <Link
            href="/share/scan"
            className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
          >
            <ScanLine className="size-4" />
            Scan ISBN
          </Link>
          <Link
            href="/wishlist"
            className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
          >
            <Heart className="size-4" />
            Wishlist
          </Link>
        </PopoverContent>
      </Popover>
    </div>
  );
}
