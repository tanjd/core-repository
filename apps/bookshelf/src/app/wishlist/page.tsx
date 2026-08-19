"use client";

import { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { toast } from "sonner";
import { Search, Plus, BookOpen, Link2, X } from "lucide-react";
import { api } from "@/lib/api";
import type {
  BookMetadataResult,
  Book,
  WishlistRequest,
  PaginatedResult,
  User,
} from "@/lib/types";
import { wishlistStatusLabel, wishlistStatusVariant } from "@/lib/wishlist";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent } from "@/components/ui/card";
import { Pagination } from "@/components/ui/Pagination";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

const PAGE_SIZE = 20;

interface SelectedBook {
  title: string;
  author: string;
  isbn: string;
  olKey: string;
  googleBooksId: string;
  coverUrl: string;
}

export default function WishlistPage() {
  const [result, setResult] = useState<PaginatedResult<WishlistRequest> | null>(
    null,
  );
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const identifiedRef = useRef(false);
  useEffect(() => {
    if (identifiedRef.current) return;
    identifiedRef.current = true;
    const stored = localStorage.getItem("bookshelf_user");
    let user: User | null = null;
    if (stored) {
      try {
        user = JSON.parse(stored);
      } catch {
        // ignore
      }
    }
    setCurrentUser(user);
  }, []);

  async function fetchRequests(q: string, p: number) {
    setLoading(true);
    setError("");
    try {
      const data = await api.getWishlistRequests({
        q: q.trim() || undefined,
        page: p,
        page_size: PAGE_SIZE,
      });
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load requests");
    } finally {
      setLoading(false);
    }
  }

  const loadedRef = useRef(false);
  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    fetchRequests("", 1);
  }, []);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setPage(1);
      fetchRequests(search, 1);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [search]);

  function handlePageChange(p: number) {
    setPage(p);
    fetchRequests(search, p);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function reload() {
    fetchRequests(search, page);
  }

  const [createOpen, setCreateOpen] = useState(false);
  const [linkTarget, setLinkTarget] = useState<WishlistRequest | null>(null);

  const requests = result?.items ?? [];
  const totalPages = result?.total_pages ?? 1;
  const total = result?.total ?? 0;
  const isAdmin = currentUser?.role === "admin";

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold">Wishlist</h1>
          <p className="text-muted-foreground text-sm">
            Books the community wants but nobody&apos;s added yet
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="size-4" />
          Add to wishlist
        </Button>
      </div>

      <div className="relative max-w-xl">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
        <Input
          type="search"
          placeholder="Search by title, author…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="pl-9 h-10"
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-28 rounded-lg" />
          ))}
        </div>
      ) : requests.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <p className="text-muted-foreground">
            {search
              ? "No matches for your search."
              : "The wishlist is empty right now."}
          </p>
        </div>
      ) : (
        <>
          <div>
            {total > 0 && (
              <p className="text-xs text-muted-foreground mb-4">
                {total} {total === 1 ? "book" : "books"} on the wishlist
              </p>
            )}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {requests.map((req) => (
                <WishlistCard
                  key={req.id}
                  request={req}
                  canManage={isAdmin || req.requester_id === currentUser?.id}
                  onCancelled={reload}
                  onLink={() => setLinkTarget(req)}
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

      <CreateRequestDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={reload}
      />

      <LinkBookDialog
        request={linkTarget}
        onOpenChange={(open) => !open && setLinkTarget(null)}
        onLinked={reload}
      />
    </div>
  );
}

function WishlistCard({
  request,
  canManage,
  onCancelled,
  onLink,
}: {
  request: WishlistRequest;
  canManage: boolean;
  onCancelled: () => void;
  onLink: () => void;
}) {
  const [cancelling, setCancelling] = useState(false);

  async function handleCancel() {
    setCancelling(true);
    try {
      await api.cancelWishlistRequest(request.id);
      toast.success("Request cancelled");
      onCancelled();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Cancel failed");
    } finally {
      setCancelling(false);
    }
  }

  return (
    <Card className="overflow-hidden py-0 gap-0">
      <CardContent className="p-3 flex gap-3">
        <div className="relative w-14 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
          {request.cover_url ? (
            <Image
              src={request.cover_url}
              alt={request.title}
              fill
              className="object-cover"
              sizes="56px"
            />
          ) : (
            <div className="flex h-full items-center justify-center">
              <BookOpen className="size-5 text-muted-foreground/60" />
            </div>
          )}
        </div>
        <div className="flex flex-col gap-1 min-w-0 flex-1">
          <p className="text-sm font-medium leading-snug line-clamp-2">
            {request.title}
          </p>
          {request.author && (
            <p className="text-xs text-muted-foreground line-clamp-1">
              {request.author}
            </p>
          )}
          <div className="flex items-center gap-2 flex-wrap mt-1">
            <Badge variant={wishlistStatusVariant[request.status]}>
              {wishlistStatusLabel[request.status]}
            </Badge>
            {request.requester?.name && (
              <span className="text-xs text-muted-foreground">
                by {request.requester.name}
              </span>
            )}
          </div>
          <div className="flex items-center gap-3 mt-1">
            <button
              onClick={onLink}
              className="text-xs text-primary hover:underline inline-flex items-center gap-1"
            >
              <Link2 className="size-3" />
              Link to a book
            </button>
            {canManage && (
              <button
                onClick={handleCancel}
                disabled={cancelling}
                className="text-xs text-muted-foreground hover:text-destructive inline-flex items-center gap-1"
              >
                <X className="size-3" />
                {cancelling ? "Cancelling…" : "Cancel"}
              </button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function CreateRequestDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<BookMetadataResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const [selected, setSelected] = useState<SelectedBook | null>(null);
  const [notes, setNotes] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [match, setMatch] = useState<WishlistRequest | null>(null);
  const [matchLoading, setMatchLoading] = useState(false);
  const [bypassMatch, setBypassMatch] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cacheRef = useRef<Map<string, BookMetadataResult[]>>(new Map());
  const clearedForShortQueryRef = useRef(false);

  // Dedup check: once a book is picked, see if it's already on someone's
  // wishlist before showing the notes/submit step, so members don't pile up
  // duplicate posts for the same title.
  useEffect(() => {
    if (!selected) return;
    let cancelled = false;
    const timer = setTimeout(() => {
      setMatchLoading(true);
      api
        .checkWishlistRequest({
          isbn: selected.isbn || undefined,
          ol_key: selected.olKey || undefined,
          google_books_id: selected.googleBooksId || undefined,
        })
        .then((data) => {
          if (!cancelled) setMatch(data.match);
        })
        .catch(() => {
          if (!cancelled) setMatch(null);
        })
        .finally(() => {
          if (!cancelled) setMatchLoading(false);
        });
    }, 0);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [selected]);

  useEffect(() => {
    if (!open) return;
    const normalized = query.trim().toLowerCase();
    if (normalized.length < 3) {
      if (!clearedForShortQueryRef.current) {
        clearedForShortQueryRef.current = true;
        setResults([]);
      }
      return;
    }
    clearedForShortQueryRef.current = false;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      if (cacheRef.current.has(normalized)) {
        setResults(cacheRef.current.get(normalized)!);
        return;
      }
      setSearching(true);
      setSearchError("");
      try {
        const data = await api.searchMetadata(normalized);
        cacheRef.current.set(normalized, data);
        setResults(data);
      } catch (err) {
        setSearchError(err instanceof Error ? err.message : "Search failed");
      } finally {
        setSearching(false);
      }
    }, 500);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, open]);

  function reset() {
    setQuery("");
    setResults([]);
    setSelected(null);
    setNotes("");
    setMatch(null);
    setBypassMatch(false);
  }

  function handleJoinExisting() {
    toast.success(
      "You're not the only one — no need to add it again. The original poster will be notified if this turns up.",
    );
    handleOpenChange(false);
  }

  function handleOpenChange(next: boolean) {
    if (!next) reset();
    onOpenChange(next);
  }

  async function handleSubmit() {
    if (!selected) return;
    setSubmitting(true);
    try {
      await api.createWishlistRequest({
        title: selected.title,
        author: selected.author,
        isbn: selected.isbn || undefined,
        ol_key: selected.olKey || undefined,
        google_books_id: selected.googleBooksId || undefined,
        cover_url: selected.coverUrl || undefined,
        notes: notes.trim() || undefined,
      });
      toast.success(
        "Added to the wishlist! We'll let you know if it turns up.",
      );
      handleOpenChange(false);
      onCreated();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to add to wishlist",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add to the wishlist</DialogTitle>
          <DialogDescription>
            Search for the book you want — we&apos;ll notify you if someone adds
            it to the catalog.
          </DialogDescription>
        </DialogHeader>

        {selected ? (
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-3 rounded-lg border p-3">
              <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
                {selected.coverUrl ? (
                  <Image
                    src={selected.coverUrl}
                    alt={selected.title}
                    fill
                    className="object-cover"
                    sizes="40px"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center">
                    <BookOpen className="size-4 text-muted-foreground/60" />
                  </div>
                )}
              </div>
              <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                <p className="text-sm font-medium truncate">{selected.title}</p>
                {selected.author && (
                  <p className="text-xs text-muted-foreground truncate">
                    {selected.author}
                  </p>
                )}
              </div>
              <button
                onClick={() => {
                  setSelected(null);
                  setMatch(null);
                  setBypassMatch(false);
                }}
                className="text-xs text-muted-foreground hover:text-foreground shrink-0"
              >
                Change
              </button>
            </div>

            {matchLoading ? (
              <Skeleton className="h-20 rounded-lg" />
            ) : match && !bypassMatch ? (
              <div className="flex flex-col gap-2 rounded-lg border bg-muted/50 p-3">
                <p className="text-sm font-medium">
                  This book&apos;s already on someone&apos;s wishlist
                </p>
                <p className="text-xs text-muted-foreground">
                  {match.requester?.name ?? "A member"} added this on{" "}
                  {new Date(match.created_at).toLocaleDateString()}
                  {match.notes ? ` — "${match.notes}"` : ""}
                </p>
                <div className="flex items-center gap-3 mt-1">
                  <Button size="sm" onClick={handleJoinExisting}>
                    I want this too
                  </Button>
                  <button
                    onClick={() => setBypassMatch(true)}
                    className="text-xs text-muted-foreground hover:text-foreground"
                  >
                    Add it separately anyway
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="wishlist-notes">
                  Notes{" "}
                  <span className="text-muted-foreground font-normal">
                    (optional)
                  </span>
                </Label>
                <Textarea
                  id="wishlist-notes"
                  className="resize-none"
                  placeholder="e.g. any edition is fine, hardcover preferred…"
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </div>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
              <Input
                type="search"
                placeholder="Search by title, author, ISBN…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="pl-9"
                autoFocus
              />
            </div>

            {searchError && (
              <p className="text-sm text-destructive">{searchError}</p>
            )}

            {searching && (
              <div className="flex flex-col gap-2">
                {[...Array(3)].map((_, i) => (
                  <Skeleton key={i} className="h-14 rounded-lg" />
                ))}
              </div>
            )}

            {!searching && results.length > 0 && (
              <div className="flex flex-col gap-2 max-h-72 overflow-y-auto">
                {results.map((r, idx) => (
                  <button
                    key={`${r.source}-${r.ol_key || r.google_books_id || idx}`}
                    onClick={() =>
                      setSelected({
                        title: r.title,
                        author: r.author,
                        isbn: r.isbn,
                        olKey: r.ol_key,
                        googleBooksId: r.google_books_id,
                        coverUrl: r.cover_url,
                      })
                    }
                    className="flex items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent transition-colors"
                  >
                    <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
                      {r.cover_url ? (
                        <Image
                          src={r.cover_url}
                          alt={r.title}
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
                      <p className="text-sm font-medium truncate">{r.title}</p>
                      {r.author && (
                        <p className="text-xs text-muted-foreground truncate">
                          {r.author}
                        </p>
                      )}
                    </div>
                    <Badge variant="secondary" className="text-[10px] shrink-0">
                      {r.source === "google_books"
                        ? "Google Books"
                        : "Open Library"}
                    </Badge>
                  </button>
                ))}
              </div>
            )}

            {!searching && query.trim().length >= 3 && results.length === 0 && (
              <p className="text-sm text-muted-foreground">
                No results found. Try a different search.
              </p>
            )}
          </div>
        )}

        <DialogFooter showCloseButton>
          {!(selected && !matchLoading && match && !bypassMatch) && (
            <Button onClick={handleSubmit} disabled={!selected || submitting}>
              {submitting ? "Adding…" : "Add to wishlist"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function LinkBookDialog({
  request,
  onOpenChange,
  onLinked,
}: {
  request: WishlistRequest | null;
  onOpenChange: (open: boolean) => void;
  onLinked: () => void;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Book[]>([]);
  const [searching, setSearching] = useState(false);
  const [linking, setLinking] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastRequestIdRef = useRef<number | null>(null);
  const clearedForShortQueryRef = useRef(false);

  useEffect(() => {
    if (!request || lastRequestIdRef.current === request.id) return;
    lastRequestIdRef.current = request.id;
    setQuery("");
    setResults([]);
  }, [request]);

  useEffect(() => {
    if (!request) return;
    const q = query.trim();
    if (q.length < 2) {
      if (!clearedForShortQueryRef.current) {
        clearedForShortQueryRef.current = true;
        setResults([]);
      }
      return;
    }
    clearedForShortQueryRef.current = false;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setSearching(true);
      try {
        const data = await api.getBooks({ q, page_size: 10 });
        setResults(data.items);
      } catch {
        setResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, request]);

  async function handleLink(book: Book) {
    if (!request) return;
    setLinking(true);
    try {
      await api.fulfillWishlistRequest(request.id, book.id);
      toast.success(`Linked to "${book.title}"`);
      onOpenChange(false);
      onLinked();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Linking failed");
    } finally {
      setLinking(false);
    }
  }

  return (
    <Dialog open={!!request} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Link to an existing catalog book</DialogTitle>
          <DialogDescription>
            {request
              ? `Find the catalog entry that fulfills "${request.title}".`
              : ""}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
            <Input
              type="search"
              placeholder="Search the catalog…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="pl-9"
              autoFocus
            />
          </div>

          {searching && (
            <div className="flex flex-col gap-2">
              {[...Array(2)].map((_, i) => (
                <Skeleton key={i} className="h-12 rounded-lg" />
              ))}
            </div>
          )}

          {!searching && results.length > 0 && (
            <div className="flex flex-col gap-2 max-h-72 overflow-y-auto">
              {results.map((book) => (
                <button
                  key={book.id}
                  onClick={() => handleLink(book)}
                  disabled={linking}
                  className="flex items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent transition-colors disabled:opacity-50"
                >
                  <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                    <p className="text-sm font-medium truncate">{book.title}</p>
                    {book.author && (
                      <p className="text-xs text-muted-foreground truncate">
                        {book.author}
                      </p>
                    )}
                  </div>
                </button>
              ))}
            </div>
          )}

          {!searching && query.trim().length >= 2 && results.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No matching catalog books found.
            </p>
          )}
        </div>

        <DialogFooter showCloseButton />
      </DialogContent>
    </Dialog>
  );
}
