"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { BookPlus, ScanLine } from "lucide-react";
import { api } from "@/lib/api";
import type { BookMetadataResult } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { BookCover } from "@/components/BookCover";
import { Breadcrumb } from "@/components/Breadcrumb";
import { MetadataSearchStep } from "./components/MetadataSearchStep";
import { ConditionPicker, type Condition } from "./components/ConditionPicker";
import { CopySettings } from "./components/CopySettings";

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

export default function SharePage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("search");

  // Auth guard
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) router.push("/login");
  }, [router]);

  // --- Step 1: Search ---
  // Pre-fill from a catalog search that came up empty (?q=...). Read via
  // window.location on mount (rather than useSearchParams) to avoid a
  // server/client hydration mismatch and the Suspense boundary that hook
  // requires — MetadataSearchStep is remounted (via key) once the prefill
  // is known, since its initialQuery prop is only read at mount. Guarded by
  // a ref (not a useState value in the deps array) so the effect only ever
  // fires its setState once, on mount.
  const prefilledRef = useRef(false);
  const [prefillQuery, setPrefillQuery] = useState<string | null>(null);
  useEffect(() => {
    if (prefilledRef.current) return;
    prefilledRef.current = true;
    setPrefillQuery(new URLSearchParams(window.location.search).get("q") ?? "");
  }, []);

  // --- Step 2: Confirm ---
  const [selected, setSelected] = useState<SelectedBook | null>(null);
  const [condition, setCondition] = useState<Condition>("good");
  const [notes, setNotes] = useState("");
  const [autoApprove, setAutoApprove] = useState(false);
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
  // Step 1: Search. MetadataSearchStep manages its own hero/results-mode
  // switch; it's keyed on prefillQuery readiness so it mounts once, after
  // the URL prefill (if any) is known — see the effect above.
  if (prefillQuery === null) return null;

  const metaChips = selected
    ? ([
        selected.isbn && `ISBN ${selected.isbn}`,
        selected.publisher &&
          (selected.publishedDate
            ? `${selected.publisher}, ${selected.publishedDate}`
            : selected.publisher),
        !selected.publisher && selected.publishedDate,
        selected.pageCount > 0 && `${selected.pageCount} pages`,
        selected.language && selected.language.toUpperCase(),
      ].filter(Boolean) as string[])
    : [];

  // All three steps stay mounted simultaneously (toggled via `hidden` rather
  // than each being its own early `return`) so MetadataSearchStep is never
  // unmounted — going "back" to search from confirm/manual used to remount
  // it fresh, silently discarding whatever query/results were already there.
  return (
    <>
      <div className={step === "manual" ? "" : "hidden"}>
        <div className="flex flex-col gap-6 max-w-lg mx-auto">
          <Breadcrumb
            back={{ onClick: () => setStep("search") }}
            backLabel="Search"
            current="Enter book manually"
          />

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
                  hideOwner={manualHideOwner}
                  onAutoApproveChange={setManualAutoApprove}
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
      </div>

      <div className={step === "confirm" && selected ? "" : "hidden"}>
        {selected && (
          <div className="flex flex-col gap-6 max-w-lg mx-auto">
            <Breadcrumb
              back={{ onClick: () => setStep("search") }}
              backLabel="Search"
              current="Confirm & share"
            />

            <div>
              <h1 className="text-2xl font-bold">Confirm & share</h1>
              <p className="text-muted-foreground text-sm mt-1">
                Review the book details and describe your copy
              </p>
            </div>

            {/* Book preview */}
            <Card>
              <CardHeader className="flex-row gap-4 items-start pb-3">
                <div className="relative w-20 aspect-[2/3] rounded overflow-hidden shrink-0 bg-muted">
                  <BookCover
                    title={selected.title}
                    author={selected.author}
                    coverUrl={selected.coverUrl}
                    sizes="80px"
                  />
                </div>
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
                hideOwner={hideOwner}
                onAutoApproveChange={setAutoApprove}
                onHideOwnerChange={setHideOwner}
              />

              <Button
                onClick={handleSubmitShare}
                disabled={submitting}
                size="lg"
              >
                <BookPlus className="size-4" />
                {submitting ? "Sharing…" : "Share this book"}
              </Button>
            </div>
          </div>
        )}
      </div>

      <div className={step === "search" ? "" : "hidden"}>
        <div className="flex flex-col gap-4 max-w-2xl mx-auto w-full">
          <MetadataSearchStep
            key={prefillQuery}
            initialQuery={prefillQuery}
            onSelect={handleSelectResult}
            onManualEntry={() => setStep("manual")}
          />

          <Link
            href="/share/scan"
            className="md:hidden flex items-center justify-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ScanLine className="size-4" />
            Scan a barcode instead
          </Link>
        </div>
      </div>
    </>
  );
}
