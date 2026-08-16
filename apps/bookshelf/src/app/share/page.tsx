"use client";

import { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { toast } from "sonner";
import { Search, ArrowLeft, BookPlus } from "lucide-react";
import { api } from "@/lib/api";
import type { BookMetadataResult } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";

type Condition = "good" | "fair" | "worn";

type Step = "search" | "confirm" | "manual";

interface SelectedBook {
  olKey: string;
  googleBooksId: string;
  source: "openlibrary" | "google_books" | "bookbrainz";
  title: string;
  author: string;
  isbn: string;
  coverUrl: string;
  description: string;
  publisher: string;
  publishedDate: string;
  pageCount: number;
  language: string;
}

const CONDITION_OPTIONS: {
  value: Condition;
  label: string;
  description: string;
}[] = [
  { value: "good", label: "Good", description: "Like new or minimal wear" },
  { value: "fair", label: "Fair", description: "Some wear, fully readable" },
  { value: "worn", label: "Worn", description: "Heavy wear but intact" },
];

export default function SharePage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("search");

  // Auth guard
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) router.push("/login");
  }, [router]);

  // --- Step 1: Search ---
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<BookMetadataResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cacheRef = useRef<Map<string, BookMetadataResult[]>>(new Map());

  useEffect(() => {
    const normalized = query.trim().toLowerCase();
    if (normalized.length < 3) {
      setSearchResults([]);
      return;
    }
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

  // --- Step 2: Confirm ---
  const [selected, setSelected] = useState<SelectedBook | null>(null);
  const [condition, setCondition] = useState<Condition>("good");
  const [notes, setNotes] = useState("");
  const [autoApprove, setAutoApprove] = useState(false);
  const [returnDateRequired, setReturnDateRequired] = useState(false);
  const [hideOwner, setHideOwner] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function handleSelectResult(result: BookMetadataResult) {
    let description = result.description;

    // For OL results, description is empty — fetch lazily
    if (result.source === "openlibrary" && !description && result.ol_key) {
      try {
        const res = await api.getOLDescription(result.ol_key);
        description = res.description;
      } catch {
        // description stays empty
      }
    }

    setSelected({
      olKey: result.ol_key,
      googleBooksId: result.google_books_id,
      source: result.source,
      title: result.title,
      author: result.author,
      isbn: result.isbn,
      coverUrl: result.cover_url,
      description,
      publisher: result.publisher,
      publishedDate: result.published_date,
      pageCount: result.page_count,
      language: result.language,
    });
    setStep("confirm");
  }

  async function handleSubmitShare() {
    if (!selected) return;
    setSubmitting(true);
    try {
      const created = await api.createBook({
        title: selected.title,
        author: selected.author,
        isbn: selected.isbn,
        ol_key: selected.olKey || undefined,
        cover_url: selected.coverUrl,
        description: selected.description,
        publisher: selected.publisher || undefined,
        published_date: selected.publishedDate || undefined,
        page_count: selected.pageCount || undefined,
        language: selected.language || undefined,
        google_books_id: selected.googleBooksId || undefined,
      });

      await api.createCopy({
        book_id: created.id,
        condition,
        notes: notes.trim() || undefined,
        auto_approve: autoApprove || undefined,
        return_date_required: returnDateRequired || undefined,
        hide_owner: hideOwner || undefined,
      });

      toast.success("Book shared! It's now in the catalog.");
      router.push("/my-books");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to share book");
    } finally {
      setSubmitting(false);
    }
  }

  // --- Manual entry ---
  const [manualTitle, setManualTitle] = useState("");
  const [manualAuthor, setManualAuthor] = useState("");
  const [manualIsbn, setManualIsbn] = useState("");
  const [manualCondition, setManualCondition] = useState<Condition>("good");
  const [manualNotes, setManualNotes] = useState("");
  const [manualAutoApprove, setManualAutoApprove] = useState(false);
  const [manualReturnDateRequired, setManualReturnDateRequired] =
    useState(false);
  const [manualHideOwner, setManualHideOwner] = useState(false);
  const [manualSubmitting, setManualSubmitting] = useState(false);

  async function handleManualSubmit() {
    if (!manualTitle.trim()) {
      toast.error("Title is required");
      return;
    }
    setManualSubmitting(true);
    try {
      const created = await api.createBook({
        title: manualTitle.trim(),
        author: manualAuthor.trim(),
        isbn: manualIsbn.trim(),
      });
      await api.createCopy({
        book_id: created.id,
        condition: manualCondition,
        notes: manualNotes.trim() || undefined,
        auto_approve: manualAutoApprove || undefined,
        return_date_required: manualReturnDateRequired || undefined,
        hide_owner: manualHideOwner || undefined,
      });
      toast.success("Book shared!");
      router.push("/my-books");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to share book");
    } finally {
      setManualSubmitting(false);
    }
  }

  // --- Render ---
  if (step === "manual") {
    return (
      <div className="flex flex-col gap-6 max-w-lg mx-auto">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setStep("search")}
          className="self-start -ml-1"
        >
          <ArrowLeft className="size-4" /> Back to search
        </Button>

        <div>
          <h1 className="text-2xl font-bold">Enter book manually</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Fill in the book details yourself
          </p>
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Title *</label>
            <Input
              value={manualTitle}
              onChange={(e) => setManualTitle(e.target.value)}
              placeholder="Book title"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Author</label>
            <Input
              value={manualAuthor}
              onChange={(e) => setManualAuthor(e.target.value)}
              placeholder="Author name"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">ISBN</label>
            <Input
              value={manualIsbn}
              onChange={(e) => setManualIsbn(e.target.value)}
              placeholder="ISBN (optional)"
            />
          </div>

          <Separator />

          <div>
            <p className="text-sm font-medium mb-3">Your copy</p>
            <div className="flex flex-col gap-4">
              <ConditionPicker
                value={manualCondition}
                onChange={setManualCondition}
              />

              <div className="flex flex-col gap-1.5">
                <label className="text-sm font-medium">
                  Notes{" "}
                  <span className="text-muted-foreground font-normal">
                    (optional)
                  </span>
                </label>
                <textarea
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  placeholder="Any notes about your copy…"
                  value={manualNotes}
                  onChange={(e) => setManualNotes(e.target.value)}
                />
              </div>

              <CopySettings
                autoApprove={manualAutoApprove}
                returnDateRequired={manualReturnDateRequired}
                hideOwner={manualHideOwner}
                onAutoApproveChange={setManualAutoApprove}
                onReturnDateRequiredChange={setManualReturnDateRequired}
                onHideOwnerChange={setManualHideOwner}
              />
            </div>
          </div>

          <Button onClick={handleManualSubmit} disabled={manualSubmitting}>
            <BookPlus className="size-4" />
            {manualSubmitting ? "Sharing…" : "Share this book"}
          </Button>
        </div>
      </div>
    );
  }

  if (step === "confirm" && selected) {
    const metaChips = [
      selected.isbn && `ISBN ${selected.isbn}`,
      selected.publisher &&
        (selected.publishedDate
          ? `${selected.publisher}, ${selected.publishedDate}`
          : selected.publisher),
      !selected.publisher && selected.publishedDate,
      selected.pageCount > 0 && `${selected.pageCount} pages`,
      selected.language && selected.language.toUpperCase(),
    ].filter(Boolean) as string[];

    return (
      <div className="flex flex-col gap-6 max-w-lg mx-auto">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setStep("search")}
          className="self-start -ml-1"
        >
          <ArrowLeft className="size-4" /> Back to search
        </Button>

        <div>
          <h1 className="text-2xl font-bold">Confirm & share</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Review the book details and describe your copy
          </p>
        </div>

        {/* Book preview */}
        <Card>
          <CardHeader className="flex-row gap-4 items-start pb-3">
            {selected.coverUrl && (
              <div className="relative w-20 aspect-[2/3] rounded overflow-hidden shrink-0 bg-muted">
                <Image
                  src={selected.coverUrl}
                  alt={selected.title}
                  fill
                  className="object-cover"
                  sizes="80px"
                />
              </div>
            )}
            <div className="flex flex-col gap-1.5 min-w-0 flex-1">
              <CardTitle className="text-base leading-snug">
                {selected.title}
              </CardTitle>
              {selected.author && (
                <CardDescription>{selected.author}</CardDescription>
              )}
              {metaChips.length > 0 && (
                <p className="text-xs text-muted-foreground">
                  {metaChips.join(" · ")}
                </p>
              )}
            </div>
          </CardHeader>
          {selected.description && (
            <CardContent className="pt-0">
              <p className="text-sm text-muted-foreground line-clamp-4">
                {selected.description}
              </p>
            </CardContent>
          )}
          <div className="px-6 pb-4">
            <button
              onClick={() => setStep("search")}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Not the right edition? Go back →
            </button>
          </div>
        </Card>

        {/* Copy settings */}
        <div className="flex flex-col gap-4">
          <div>
            <p className="text-sm font-semibold">Your copy</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Describe the physical copy you&apos;re sharing
            </p>
          </div>

          <ConditionPicker value={condition} onChange={setCondition} />

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">
              Notes{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <textarea
              className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              placeholder="e.g. spine slightly creased, all pages intact…"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
          </div>

          <CopySettings
            autoApprove={autoApprove}
            returnDateRequired={returnDateRequired}
            hideOwner={hideOwner}
            onAutoApproveChange={setAutoApprove}
            onReturnDateRequiredChange={setReturnDateRequired}
            onHideOwnerChange={setHideOwner}
          />

          <Button onClick={handleSubmitShare} disabled={submitting} size="lg">
            <BookPlus className="size-4" />
            {submitting ? "Sharing…" : "Share this book"}
          </Button>
        </div>
      </div>
    );
  }

  // Step 1: Search — hero mode when idle, results mode when typing
  const showHero =
    query.trim().length < 3 && !searching && searchResults.length === 0;

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
            autoFocus
          />
        </div>

        <button
          onClick={() => setStep("manual")}
          className="text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          Can&apos;t find your book? Enter manually →
        </button>
      </div>
    );
  }

  // Results mode
  return (
    <div className="flex flex-col gap-4 max-w-2xl mx-auto w-full">
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
              onClick={() => handleSelectResult(result)}
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
            Metadata providers may be temporarily unavailable. You can still add
            your book manually below.
          </p>
        </div>
      )}

      <div className="border-t pt-4">
        <button
          onClick={() => setStep("manual")}
          className="text-sm text-primary hover:underline"
        >
          Can&apos;t find your book? Enter manually →
        </button>
      </div>
    </div>
  );
}

function ConditionPicker({
  value,
  onChange,
}: {
  value: Condition;
  onChange: (v: Condition) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium">Condition</label>
      <div className="flex gap-2">
        {CONDITION_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={`flex-1 flex flex-col items-center gap-0.5 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
              value === opt.value
                ? "border-primary bg-primary/5 text-primary font-medium"
                : "border-input text-muted-foreground hover:bg-accent hover:text-foreground"
            }`}
          >
            <span className="font-medium text-sm">{opt.label}</span>
            <span className="text-[11px] leading-tight text-center opacity-80">
              {opt.description}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

function CopySettings({
  autoApprove,
  returnDateRequired,
  hideOwner,
  onAutoApproveChange,
  onReturnDateRequiredChange,
  onHideOwnerChange,
}: {
  autoApprove: boolean;
  returnDateRequired: boolean;
  hideOwner: boolean;
  onAutoApproveChange: (v: boolean) => void;
  onReturnDateRequiredChange: (v: boolean) => void;
  onHideOwnerChange: (v: boolean) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      {[
        {
          id: "auto-approve",
          label: "Auto-approve requests",
          description:
            "Loan requests are accepted automatically without your review",
          checked: autoApprove,
          onChange: onAutoApproveChange,
        },
        {
          id: "return-date",
          label: "Require return date",
          description:
            "Borrowers must specify when they plan to return the book",
          checked: returnDateRequired,
          onChange: onReturnDateRequiredChange,
        },
        {
          id: "hide-owner",
          label: "Stay anonymous",
          description: "Your name is hidden from borrowers in the catalog",
          checked: hideOwner,
          onChange: onHideOwnerChange,
        },
      ].map(({ id, label, description, checked, onChange }) => (
        <div key={id} className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm font-medium">{label}</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              {description}
            </p>
          </div>
          <Switch id={id} checked={checked} onCheckedChange={onChange} />
        </div>
      ))}
    </div>
  );
}
