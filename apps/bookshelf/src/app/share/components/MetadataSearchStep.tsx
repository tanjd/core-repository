"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, Info, Search } from "lucide-react";
import { api } from "@/lib/api";
import type { BookMetadataResult } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { BookCover } from "@/components/BookCover";

/** One row in the rendered results list — a single edition, or a bucket of editions of the same work. */
type ResultDisplayItem =
  | { kind: "single"; result: BookMetadataResult }
  | { kind: "bucket"; workKey: string; editions: BookMetadataResult[] };

/**
 * Groups already-score-sorted results by `work_key` — a non-empty key shared by
 * two or more results becomes one bucketed card; everything else renders as a
 * single row, same as before bucketing existed.
 */
function bucketResults(results: BookMetadataResult[]): ResultDisplayItem[] {
  const byWorkKey = new Map<string, BookMetadataResult[]>();
  for (const result of results) {
    if (!result.work_key) continue;
    const group = byWorkKey.get(result.work_key);
    if (group) group.push(result);
    else byWorkKey.set(result.work_key, [result]);
  }

  const seenWorkKeys = new Set<string>();
  const items: ResultDisplayItem[] = [];
  for (const result of results) {
    const editions = result.work_key
      ? byWorkKey.get(result.work_key)
      : undefined;
    if (editions && editions.length > 1) {
      if (seenWorkKeys.has(result.work_key!)) continue;
      seenWorkKeys.add(result.work_key!);
      items.push({ kind: "bucket", workKey: result.work_key!, editions });
    } else {
      items.push({ kind: "single", result });
    }
  }
  return items;
}

function sourceLabel(source: BookMetadataResult["source"]) {
  if (source === "google_books") return "Google Books";
  if (source === "openlibrary") return "Open Library";
  return "BookBrainz";
}

function EditionRow({
  result,
  onSelect,
}: {
  result: BookMetadataResult;
  onSelect: (result: BookMetadataResult) => void;
}) {
  const descriptionEnriched = result.enriched_fields?.includes("description");
  return (
    <button
      onClick={() => onSelect(result)}
      className="flex items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent transition-colors w-full"
    >
      <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
        <BookCover
          title={result.title}
          author={result.author}
          coverUrl={result.cover_url}
          sizes="40px"
        />
      </div>
      <div className="flex flex-col gap-0.5 min-w-0 flex-1">
        <p className="text-sm font-medium truncate">{result.title}</p>
        {result.author && (
          <p className="text-xs text-muted-foreground truncate">
            {result.author}
          </p>
        )}
        <p className="text-xs text-muted-foreground truncate">
          {[result.publisher, result.published_date, result.isbn]
            .filter(Boolean)
            .join(" · ")}
        </p>
        {descriptionEnriched && (
          <p className="text-[10px] text-muted-foreground italic">
            Description from another edition
          </p>
        )}
      </div>
      <Badge variant="secondary" className="text-[10px] shrink-0">
        {sourceLabel(result.source)}
      </Badge>
    </button>
  );
}

function BucketedResultCard({
  editions,
  onSelect,
}: {
  editions: BookMetadataResult[];
  onSelect: (result: BookMetadataResult) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const representative = editions[0];
  const descriptionEnriched =
    representative.enriched_fields?.includes("description");

  return (
    <div className="rounded-lg border overflow-hidden">
      <button
        onClick={() => setExpanded((v) => !v)}
        className="flex items-center gap-3 p-3 text-left hover:bg-accent transition-colors w-full"
      >
        <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
          <BookCover
            title={representative.title}
            author={representative.author}
            coverUrl={representative.cover_url}
            sizes="40px"
          />
        </div>
        <div className="flex flex-col gap-0.5 min-w-0 flex-1">
          <p className="text-sm font-medium truncate">{representative.title}</p>
          {representative.author && (
            <p className="text-xs text-muted-foreground truncate">
              {representative.author}
            </p>
          )}
          {descriptionEnriched && (
            <p className="text-[10px] text-muted-foreground italic">
              Description from another edition
            </p>
          )}
        </div>
        <Badge variant="secondary" className="text-[10px] shrink-0">
          {editions.length} editions
        </Badge>
        <ChevronDown
          className={`size-4 text-muted-foreground shrink-0 transition-transform ${
            expanded ? "rotate-180" : ""
          }`}
        />
      </button>
      {expanded && (
        <div className="flex flex-col gap-2 border-t p-3 bg-muted/30">
          {editions.map((edition, idx) => (
            <EditionRow
              key={`${edition.source}-${edition.ol_key || edition.google_books_id || idx}`}
              result={edition}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </div>
  );
}

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
  const resultItems = useMemo(
    () => bucketResults(searchResults),
    [searchResults],
  );
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
          {resultItems.some((item) => item.kind === "bucket") && (
            <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
              <Info className="size-3.5 shrink-0 mt-0.5" />
              We group listings for the same book and fill in missing details
              from other editions when we&apos;re confident it&apos;s the same
              title.
            </p>
          )}
          {resultItems.map((item, idx) =>
            item.kind === "bucket" ? (
              <BucketedResultCard
                key={`bucket-${item.workKey}`}
                editions={item.editions}
                onSelect={onSelect}
              />
            ) : (
              <EditionRow
                key={`${item.result.source}-${item.result.ol_key || item.result.google_books_id || idx}`}
                result={item.result}
                onSelect={onSelect}
              />
            ),
          )}
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
