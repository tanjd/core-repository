"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { BrowserMultiFormatReader, BarcodeFormat } from "@zxing/browser";
import type { IScannerControls } from "@zxing/browser";
import { ArrowLeft, ListChecks, Search, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import type { BookMetadataResult } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ConditionPicker, type Condition } from "../components/ConditionPicker";
import { CopySettings } from "../components/CopySettings";
import { MetadataSearchStep } from "../components/MetadataSearchStep";
import { BookCover } from "@/components/BookCover";

type ScanItemStatus = "resolving" | "resolved" | "unresolved";

interface ScanItem {
  id: string;
  isbn: string;
  status: ScanItemStatus;
  result?: BookMetadataResult;
  condition: Condition;
  notes: string;
  autoApprove: boolean;
  hideOwner: boolean;
}

// Books carry EAN-13 (ISBN-13) barcodes, occasionally an older UPC-A —
// scoping the reader to just these two both speeds up/improves decode
// accuracy and avoids false-positive matches against unrelated barcodes.
function normalizeIsbn(text: string) {
  return text.replace(/[^0-9Xx]/g, "").toUpperCase();
}

const SAME_CODE_COOLDOWN_MS = 2000;

export default function ScanPage() {
  const router = useRouter();

  // Auth guard
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) router.push("/login");
  }, [router]);

  const [view, setView] = useState<"scan" | "review">("scan");
  const [items, setItems] = useState<ScanItem[]>([]);
  const [cameraError, setCameraError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [manualTargetId, setManualTargetId] = useState<string | null>(null);

  const videoRef = useRef<HTMLVideoElement>(null);
  const controlsRef = useRef<IScannerControls | null>(null);
  // Serializes camera setup attempts on the shared <video> element — React's
  // dev-only Strict Mode double-invokes this effect (mount, cleanup, mount
  // again) faster than the async getUserMedia()/attach/play() chain
  // resolves, so a naive effect races two setups against the same element
  // and one play() call aborts the other. Each invocation awaits the prior
  // one's full settle (by which point its cancelled flag already stopped
  // it) before starting its own, so setups never overlap.
  const setupRef = useRef<Promise<void>>(Promise.resolve());
  const lastDecodedRef = useRef<{ text: string; at: number } | null>(null);
  const inFlightRef = useRef<Set<string>>(new Set());
  // Tracks which already-in-batch ISBNs we've already toasted about, so
  // holding the camera on the same physical book doesn't re-fire the
  // "Already scanned" toast every SAME_CODE_COOLDOWN_MS forever — cleared
  // when an item is removed, so a deliberately-removed book can be rescanned.
  const dedupToastedRef = useRef<Set<string>>(new Set());
  const itemsRef = useRef<ScanItem[]>([]);
  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  const addItemAndResolve = useCallback(async (isbn: string) => {
    const id = crypto.randomUUID();
    setItems((prev) => [
      ...prev,
      {
        id,
        isbn,
        status: "resolving",
        condition: "good",
        notes: "",
        autoApprove: false,
        hideOwner: false,
      },
    ]);
    try {
      const results = await api.searchMetadata(isbn);
      inFlightRef.current.delete(isbn);
      const best = results[0];
      setItems((prev) =>
        prev.map((it) =>
          it.id === id
            ? best
              ? { ...it, status: "resolved" as const, result: best }
              : { ...it, status: "unresolved" as const }
            : it,
        ),
      );
    } catch {
      inFlightRef.current.delete(isbn);
      setItems((prev) =>
        prev.map((it) =>
          it.id === id ? { ...it, status: "unresolved" as const } : it,
        ),
      );
    }
  }, []);

  // Client-side analogue of backend rate limiting (there is none on
  // /books/metadata/search) — decodeFromVideoDevice's callback fires on
  // every decode attempt per frame, not once per unique code, so a barcode
  // held steady in view would otherwise fire dozens of lookups per second.
  const handleDecode = useCallback(
    (text: string) => {
      const isbn = normalizeIsbn(text);
      if (!isbn) return;
      const now = Date.now();
      const last = lastDecodedRef.current;
      if (last && last.text === isbn && now - last.at < SAME_CODE_COOLDOWN_MS) {
        return;
      }
      lastDecodedRef.current = { text: isbn, at: now };

      if (
        inFlightRef.current.has(isbn) ||
        itemsRef.current.some((it) => it.isbn === isbn)
      ) {
        if (!dedupToastedRef.current.has(isbn)) {
          dedupToastedRef.current.add(isbn);
          toast.info(`Already scanned ${isbn}`);
        }
        return;
      }
      inFlightRef.current.add(isbn);
      addItemAndResolve(isbn);
    },
    [addItemAndResolve],
  );

  // Camera + decode loop — only runs while the viewfinder view is active,
  // so switching to the review list releases the camera.
  useEffect(() => {
    if (view !== "scan") return;
    let cancelled = false;

    const previousSetup = setupRef.current;
    const thisSetup = previousSetup
      .catch(() => {
        // A prior invocation's own failure shouldn't block this one.
      })
      .then(async () => {
        if (cancelled) return;
        const reader = new BrowserMultiFormatReader();
        reader.possibleFormats = [BarcodeFormat.EAN_13, BarcodeFormat.UPC_A];
        try {
          const controls = await reader.decodeFromVideoDevice(
            undefined,
            videoRef.current ?? undefined,
            (result) => {
              if (cancelled || !result) return;
              handleDecode(result.getText());
            },
          );
          if (cancelled) {
            controls.stop();
            return;
          }
          controlsRef.current = controls;
          // Clear any error from a previous attempt now that the camera has
          // started successfully (e.g. the user retried after a denial).
          setCameraError("");
        } catch (err) {
          if (!cancelled) {
            setCameraError(
              err instanceof Error
                ? err.message
                : "Could not access the camera",
            );
          }
        }
      });
    setupRef.current = thisSetup;

    return () => {
      cancelled = true;
      controlsRef.current?.stop();
      controlsRef.current = null;
    };
  }, [view, handleDecode]);

  function handleRemove(id: string) {
    setItems((prev) => {
      const removed = prev.find((it) => it.id === id);
      if (removed) {
        inFlightRef.current.delete(removed.isbn);
        dedupToastedRef.current.delete(removed.isbn);
      }
      return prev.filter((it) => it.id !== id);
    });
  }

  function updateItem(id: string, patch: Partial<ScanItem>) {
    setItems((prev) =>
      prev.map((it) => (it.id === id ? { ...it, ...patch } : it)),
    );
  }

  function handleManualResolve(result: BookMetadataResult) {
    if (!manualTargetId) return;
    updateItem(manualTargetId, { status: "resolved", result });
    setManualTargetId(null);
  }

  async function handleSubmitBatch() {
    const toSubmit = items.filter(
      (it): it is ScanItem & { result: BookMetadataResult } =>
        it.status === "resolved" && !!it.result,
    );
    if (toSubmit.length === 0) return;
    setSubmitting(true);

    const outcomes = await Promise.allSettled(
      toSubmit.map(async (item) => {
        const r = item.result;
        const created = await api.createBook({
          title: r.title,
          author: r.author,
          isbn: r.isbn,
          ol_key: r.ol_key || undefined,
          cover_url: r.cover_url,
          description: r.description,
          publisher: r.publisher || undefined,
          published_date: r.published_date || undefined,
          page_count: r.page_count || undefined,
          language: r.language || undefined,
          google_books_id: r.google_books_id || undefined,
        });
        await api.createCopy({
          book_id: created.id,
          condition: item.condition,
          notes: item.notes.trim() || undefined,
          auto_approve: item.autoApprove || undefined,
          hide_owner: item.hideOwner || undefined,
        });
        return item.id;
      }),
    );

    const succeededIds = new Set(
      outcomes
        .filter(
          (o): o is PromiseFulfilledResult<string> => o.status === "fulfilled",
        )
        .map((o) => o.value),
    );
    const failedCount = outcomes.length - succeededIds.size;

    if (failedCount === 0) {
      toast.success(
        `Added ${succeededIds.size} book${succeededIds.size === 1 ? "" : "s"} to the catalog!`,
      );
      router.push("/my-books");
    } else {
      toast.error(
        `${succeededIds.size} added, ${failedCount} failed — retry the failed ones below`,
      );
      setItems((prev) => prev.filter((it) => !succeededIds.has(it.id)));
    }
    setSubmitting(false);
  }

  const manualTarget = items.find((it) => it.id === manualTargetId) ?? null;

  if (view === "review") {
    return (
      <div className="flex flex-col gap-6 max-w-2xl mx-auto pb-24">
        <div className="flex items-center justify-between">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setView("scan")}
            className="self-start -ml-1"
          >
            <ArrowLeft className="size-4" /> Back to scanning
          </Button>
        </div>

        <div>
          <h1 className="text-2xl font-bold">Review scanned books</h1>
          <p className="text-muted-foreground text-sm mt-1">
            {items.length} book{items.length === 1 ? "" : "s"} scanned — adjust
            details or remove any you don&apos;t want to add
          </p>
        </div>

        {items.length === 0 && (
          <p className="text-sm text-muted-foreground">
            Nothing scanned yet.{" "}
            <button
              onClick={() => setView("scan")}
              className="text-primary hover:underline"
            >
              Go back to scanning
            </button>
            .
          </p>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {items.map((item) => (
            <ScanItemCard
              key={item.id}
              item={item}
              onRemove={() => handleRemove(item.id)}
              onSearchManually={() => setManualTargetId(item.id)}
              onChange={(patch) => updateItem(item.id, patch)}
            />
          ))}
        </div>

        <div
          className="fixed inset-x-0 bottom-0 z-[60] border-t bg-background p-4 md:pb-4"
          style={{ paddingBottom: "calc(env(safe-area-inset-bottom) + 1rem)" }}
        >
          <div className="max-w-2xl mx-auto">
            <Button
              onClick={handleSubmitBatch}
              disabled={
                submitting ||
                items.filter((it) => it.status === "resolved").length === 0
              }
              size="lg"
              className="w-full"
            >
              {submitting
                ? "Adding books…"
                : `Add ${items.filter((it) => it.status === "resolved").length} book${
                    items.filter((it) => it.status === "resolved").length === 1
                      ? ""
                      : "s"
                  } to the catalog`}
            </Button>
          </div>
        </div>

        <Dialog
          open={manualTarget !== null}
          onOpenChange={(open) => !open && setManualTargetId(null)}
        >
          <DialogContent className="max-h-[85vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Search for this book</DialogTitle>
            </DialogHeader>
            {manualTarget && (
              <MetadataSearchStep
                key={manualTarget.id}
                variant="compact"
                initialQuery={manualTarget.isbn}
                autoFocus={false}
                onSelect={handleManualResolve}
              />
            )}
          </DialogContent>
        </Dialog>
      </div>
    );
  }

  // Viewfinder view
  return (
    <div className="fixed inset-0 z-[60] flex flex-col bg-background">
      <div className="flex items-center justify-between p-4">
        <Link
          href="/share"
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> Done scanning
        </Link>
        <span className="text-sm font-medium">{items.length} scanned</span>
      </div>

      <div className="relative flex-1 mx-4 mb-4 rounded-lg overflow-hidden bg-muted">
        {cameraError ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-6 text-center">
            <p className="text-sm font-medium">Camera unavailable</p>
            <p className="text-sm text-muted-foreground">{cameraError}</p>
            <Link
              href="/share"
              className="text-sm text-primary hover:underline"
            >
              Search manually instead
            </Link>
          </div>
        ) : (
          <video
            ref={videoRef}
            className="absolute inset-0 size-full object-cover"
            muted
            playsInline
          />
        )}
      </div>

      <div
        className="p-4 flex flex-col gap-2"
        style={{ paddingBottom: "calc(env(safe-area-inset-bottom) + 1rem)" }}
      >
        <Button
          onClick={() => setView("review")}
          disabled={items.length === 0}
          size="lg"
        >
          <ListChecks className="size-4" />
          Review &amp; submit ({items.length})
        </Button>
      </div>
    </div>
  );
}

function ScanItemCard({
  item,
  onRemove,
  onSearchManually,
  onChange,
}: {
  item: ScanItem;
  onRemove: () => void;
  onSearchManually: () => void;
  onChange: (patch: Partial<ScanItem>) => void;
}) {
  const r = item.result;

  return (
    <div className="flex flex-col gap-3 rounded-lg border p-3">
      <div className="flex items-start gap-3">
        <div className="relative w-12 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
          {item.status === "resolving" && !r?.cover_url ? (
            <div className="flex h-full items-center justify-center text-[8px] text-muted-foreground text-center">
              …
            </div>
          ) : (
            <BookCover
              title={r?.title ?? item.isbn}
              author={r?.author}
              coverUrl={r?.cover_url}
              sizes="48px"
            />
          )}
        </div>
        <div className="flex flex-col gap-0.5 min-w-0 flex-1">
          {item.status === "resolving" && (
            <p className="text-sm text-muted-foreground">
              Looking up {item.isbn}…
            </p>
          )}
          {item.status === "resolved" && r && (
            <>
              <p className="text-sm font-medium truncate">{r.title}</p>
              {r.author && (
                <p className="text-xs text-muted-foreground truncate">
                  {r.author}
                </p>
              )}
            </>
          )}
          {item.status === "unresolved" && (
            <>
              <p className="text-sm font-medium">No match for {item.isbn}</p>
              <button
                onClick={onSearchManually}
                className="text-xs text-primary hover:underline flex items-center gap-1 w-fit"
              >
                <Search className="size-3" />
                Search manually
              </button>
            </>
          )}
        </div>
        <button
          onClick={onRemove}
          aria-label="Remove from batch"
          className="text-muted-foreground hover:text-destructive p-1 -m-1"
        >
          <Trash2 className="size-4" />
        </button>
      </div>

      {item.status === "resolved" && (
        <details className="text-sm">
          <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
            Copy details
          </summary>
          <div className="flex flex-col gap-3 pt-3">
            <ConditionPicker
              value={item.condition}
              onChange={(condition) => onChange({ condition })}
            />
            <textarea
              className="flex min-h-[60px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              placeholder="Notes about this copy (optional)…"
              value={item.notes}
              onChange={(e) => onChange({ notes: e.target.value })}
            />
            <CopySettings
              autoApprove={item.autoApprove}
              hideOwner={item.hideOwner}
              onAutoApproveChange={(autoApprove) => onChange({ autoApprove })}
              onHideOwnerChange={(hideOwner) => onChange({ hideOwner })}
            />
          </div>
        </details>
      )}
    </div>
  );
}
