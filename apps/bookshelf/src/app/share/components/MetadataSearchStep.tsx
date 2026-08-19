"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import { Search } from "lucide-react";
import { api } from "@/lib/api";
import type { BookMetadataResult } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Title/author/ISBN metadata search — shared by the main /share flow and
 * /share/scan's per-item "search manually" fallback for an unresolved scan.
 * ISBN needs no special handling here: the backend's /books/metadata/search
 * already treats it as free text (Open Library/Google Books/BookBrainz all
 * match on ISBN natively).
 */
export function MetadataSearchStep({
  initialQuery = "",
  onSelect,
  onManualEntry,
  variant = "hero",
  autoFocus = true,
}: {
  initialQuery?: string;
  onSelect: (result: BookMetadataResult) => void;
  onManualEntry?: () => void;
  variant?: "hero" | "compact";
  autoFocus?: boolean;
}) {
  const [query, setQuery] = useState(initialQuery);
  const [searchResults, setSearchResults] = useState<BookMetadataResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cacheRef = useRef<Map<string, BookMetadataResult[]>>(new Map());
  const clearedForShortQueryRef = useRef(false);

  useEffect(() => {
    const normalized = query.trim().toLowerCase();
    if (normalized.length < 3) {
      if (!clearedForShortQueryRef.current) {
        clearedForShortQueryRef.current = true;
        setSearchResults([]);
      }
      return;
    }
    clearedForShortQueryRef.current = false;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      if (cacheRef.current.has(normalized)) {
        setSearchResults(cacheRef.current.get(normalized)!);
        return;
      }
      setSearching(true);
      setSearchError("");
      try {
        const results = await api.searchMetadata(normalized);
        cacheRef.current.set(normalized, results);
        setSearchResults(results);
      } catch (err) {
        setSearchError(err instanceof Error ? err.message : "Search failed");
      } finally {
        setSearching(false);
      }
    }, 500);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query]);

  const showHero =
    variant === "hero" &&
    query.trim().length < 3 &&
    !searching &&
    searchResults.length === 0;

  if (showHero) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[45vh] gap-8 px-4">
        <div className="flex flex-col items-center gap-2 text-center">
          <h1 className="text-3xl font-bold">Share a Book</h1>
          <p className="text-muted-foreground">
            Search by title, author, or ISBN
          </p>
        </div>

        <div className="relative w-full max-w-xl">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 size-5 text-muted-foreground pointer-events-none" />
          <Input
            type="search"
            placeholder="Search by title, author, ISBN…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="pl-12 h-12 rounded-full shadow-sm text-base"
            autoFocus={autoFocus}
          />
        </div>

        {onManualEntry && (
          <button
            onClick={onManualEntry}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            Can&apos;t find your book? Enter manually →
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 w-full">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
        <Input
          type="search"
          placeholder="Search by title, author, ISBN…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="pl-9"
          autoFocus={autoFocus}
        />
      </div>

      {searchError && <p className="text-sm text-destructive">{searchError}</p>}

      {searching && (
        <div className="flex flex-col gap-2">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-3 rounded-lg border p-3"
            >
              <Skeleton
                className="w-10 shrink-0 rounded"
                style={{ aspectRatio: "2/3" }}
              />
              <div className="flex flex-col gap-1.5 flex-1">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      )}

      {!searching && searchResults.length > 0 && (
        <div className="flex flex-col gap-2">
          {searchResults.map((result, idx) => (
            <button
              key={`${result.source}-${result.ol_key || result.google_books_id || idx}`}
              onClick={() => onSelect(result)}
              className="flex items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent transition-colors"
            >
              <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
                {result.cover_url ? (
                  <Image
                    src={result.cover_url}
                    alt={result.title}
                    fill
                    className="object-cover"
                    sizes="40px"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-[8px] text-muted-foreground text-center">
                    No cover
                  </div>
                )}
              </div>
              <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                <p className="text-sm font-medium truncate">{result.title}</p>
                {result.author && (
                  <p className="text-xs text-muted-foreground truncate">
                    {result.author}
                  </p>
                )}
              </div>
              <Badge variant="secondary" className="text-[10px] shrink-0">
                {result.source === "google_books"
                  ? "Google Books"
                  : "Open Library"}
              </Badge>
            </button>
          ))}
        </div>
      )}

      {!searching && query.trim().length >= 3 && searchResults.length === 0 && (
        <div className="flex flex-col gap-1">
          <p className="text-sm text-muted-foreground">No results found.</p>
          <p className="text-xs text-muted-foreground">
            Metadata providers may be temporarily unavailable
            {onManualEntry
              ? " — you can still add your book manually below."
              : "."}
          </p>
        </div>
      )}

      {onManualEntry && (
        <div className="border-t pt-4">
          <button
            onClick={onManualEntry}
            className="text-sm text-primary hover:underline"
          >
            Can&apos;t find your book? Enter manually →
          </button>
        </div>
      )}
    </div>
  );
}
