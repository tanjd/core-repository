"use client";

import { useCallback, useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  Plus,
  Pencil,
  Trash2,
  ArrowRightLeft,
  Download,
  Upload,
  Search,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { api, downloadMyCopiesExport } from "@/lib/api";
import type {
  MyCopiesExportFormat,
  ImportResult,
  ImportRowAction,
  ImportSummary,
  ImportDecision,
} from "@/lib/api";
import type { Copy } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { BookCover } from "@/components/BookCover";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";

type Condition = "good" | "fair" | "worn";

interface MyCopy extends Copy {
  bookTitle?: string;
  bookAuthor?: string;
  bookCoverUrl?: string;
}

interface BookGroup {
  bookId: number;
  title: string;
  author: string;
  coverUrl: string;
  copies: MyCopy[];
}

interface ActiveLoan {
  borrowerName: string;
  dueDate?: string;
}

const conditionVariant: Record<string, "default" | "secondary" | "outline"> = {
  good: "default",
  fair: "secondary",
  worn: "outline",
};

const statusVariant: Record<
  string,
  "success" | "secondary" | "destructive" | "outline"
> = {
  available: "success",
  unavailable: "secondary",
  loaned: "destructive",
  requested: "outline",
};

const importActionVariant: Record<
  ImportRowAction,
  "success" | "outline" | "secondary"
> = {
  create_book: "success",
  match_existing_book: "outline",
  possible_match: "secondary",
  skipped: "secondary",
};

const importActionLabel: Record<ImportRowAction, string> = {
  create_book: "New book",
  match_existing_book: "Matched",
  possible_match: "Possible match",
  skipped: "Skipped",
};

const SORT_LABELS: Record<string, string> = {
  title: "Title A–Z",
  author: "Author A–Z",
  copies: "Most Copies",
  newest: "Recently Added",
};

function importSummaryText(summary: ImportSummary, isResult: boolean): string {
  const parts: string[] = [];
  if (summary.books_created > 0) {
    parts.push(
      `${summary.books_created} new book${summary.books_created === 1 ? "" : "s"}`,
    );
  }
  if (summary.books_matched > 0) {
    parts.push(`${summary.books_matched} matched to your existing catalog`);
  }
  if (summary.possible_matches > 0) {
    parts.push(
      `${summary.possible_matches} possible match${summary.possible_matches === 1 ? "" : "es"} to review`,
    );
  }
  if (summary.skipped > 0) {
    parts.push(`${summary.skipped} skipped`);
  }
  if (parts.length === 0) return "Nothing to import";
  return `${isResult ? "Imported" : "Will import"}: ${parts.join(", ")}`;
}

export default function MyBooksPage() {
  const router = useRouter();
  const [bookGroups, setBookGroups] = useState<BookGroup[]>([]);
  const [pendingCounts, setPendingCounts] = useState<Record<number, number>>(
    {},
  );
  const [activeLoans, setActiveLoans] = useState<Record<number, ActiveLoan>>(
    {},
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<"title" | "author" | "copies" | "newest">(
    "title",
  );
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [conditionFilter, setConditionFilter] = useState<string>("all");

  // Edit dialog
  const [editCopy, setEditCopy] = useState<MyCopy | null>(null);
  const [editCondition, setEditCondition] = useState<Condition>("good");
  const [editNotes, setEditNotes] = useState("");
  const [editStatus, setEditStatus] = useState<string>("available");
  const [editAutoApprove, setEditAutoApprove] = useState(false);
  const [editReturnDateRequired, setEditReturnDateRequired] = useState(false);
  const [editHideOwner, setEditHideOwner] = useState(false);
  const [editSubmitting, setEditSubmitting] = useState(false);

  // Transfer dialog
  const [transferCopy, setTransferCopy] = useState<MyCopy | null>(null);
  const [transferEmail, setTransferEmail] = useState("");
  const [transferSubmitting, setTransferSubmitting] = useState(false);

  // Delete confirm dialog
  const [deleteCopy, setDeleteCopy] = useState<MyCopy | null>(null);
  const [deleteSubmitting, setDeleteSubmitting] = useState(false);

  // Export dialog
  const [exportOpen, setExportOpen] = useState(false);
  const [exportFormat, setExportFormat] =
    useState<MyCopiesExportFormat>("json");
  const [exportSubmitting, setExportSubmitting] = useState(false);

  // Import dialog
  const [importOpen, setImportOpen] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importPreview, setImportPreview] = useState<ImportResult | null>(null);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [importBusy, setImportBusy] = useState(false);
  const [importError, setImportError] = useState("");
  // Per-row resolution for possible_match rows, keyed by 1-based row number
  // (importRowResult.row). A row with no entry here defaults to "create_new"
  // on commit — matches the backend's safe default.
  const [importDecisions, setImportDecisions] = useState<
    Record<number, ImportDecision>
  >({});
  const importInputRef = useRef<HTMLInputElement>(null);

  // Fetched separately, after the main copies list renders — per-copy
  // pending-request counts and active-loan details aren't returned by
  // GET /copies/mine, and this shouldn't block the page's primary loading
  // state.
  const loadRequestInfo = useCallback(async (copies: Copy[]) => {
    const results = await Promise.all(
      copies.map((copy) =>
        api
          .getLoanRequestsByCopy(copy.id)
          .then((reqs) => [copy.id, reqs] as const)
          .catch(() => [copy.id, []] as const),
      ),
    );
    const pending: Record<number, number> = {};
    const active: Record<number, ActiveLoan> = {};
    for (const [copyId, reqs] of results) {
      pending[copyId] = reqs.filter((r) => r.status === "pending").length;
      const accepted = reqs.find((r) => r.status === "accepted");
      if (accepted) {
        active[copyId] = {
          borrowerName:
            accepted.borrower?.name ?? `User #${accepted.borrower_id}`,
          dueDate: accepted.expected_return_date,
        };
      }
    }
    setPendingCounts(pending);
    setActiveLoans(active);
  }, []);

  const loadMyCopies = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const copies = await api.getMyCopies();
      const groupMap = new Map<number, BookGroup>();
      for (const copy of copies) {
        const book = copy.book;
        if (!book) continue;
        const enriched: MyCopy = {
          ...copy,
          bookTitle: book.title,
          bookAuthor: book.author,
          bookCoverUrl: book.cover_url,
        };
        if (!groupMap.has(book.id)) {
          groupMap.set(book.id, {
            bookId: book.id,
            title: book.title,
            author: book.author ?? "",
            coverUrl: book.cover_url ?? "",
            copies: [],
          });
        }
        groupMap.get(book.id)!.copies.push(enriched);
      }
      setBookGroups([...groupMap.values()]);
      loadRequestInfo(copies);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load your books",
      );
    } finally {
      setLoading(false);
    }
  }, [loadRequestInfo]);

  const checkedRef = useRef(false);
  useEffect(() => {
    if (checkedRef.current) return;
    checkedRef.current = true;
    const token = localStorage.getItem("bookshelf_token");
    const stored = token ? localStorage.getItem("bookshelf_user") : null;
    let validUser = false;
    if (stored) {
      try {
        JSON.parse(stored); // validate JSON
        validUser = true;
      } catch {
        validUser = false;
      }
    }
    if (!token || !validUser) {
      router.push("/login");
      return;
    }
    loadMyCopies();
  }, [router, loadMyCopies]);

  function openEdit(copy: MyCopy) {
    setEditCopy(copy);
    setEditCondition(copy.condition as Condition);
    setEditNotes(copy.notes ?? "");
    setEditStatus(copy.status);
    setEditAutoApprove(copy.auto_approve ?? false);
    setEditReturnDateRequired(copy.return_date_required ?? false);
    setEditHideOwner(copy.hide_owner ?? false);
  }

  async function handleEditSave() {
    if (!editCopy) return;
    setEditSubmitting(true);
    try {
      await api.updateCopy(editCopy.id, {
        condition: editCondition,
        notes: editNotes.trim(),
        status: editStatus,
        auto_approve: editAutoApprove,
        return_date_required: editReturnDateRequired,
        hide_owner: editHideOwner,
      });
      toast.success("Copy updated");
      setEditCopy(null);
      loadMyCopies();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Update failed");
    } finally {
      setEditSubmitting(false);
    }
  }

  async function handleDelete() {
    if (!deleteCopy) return;
    setDeleteSubmitting(true);
    try {
      await api.deleteCopy(deleteCopy.id);
      toast.success("Copy removed");
      setDeleteCopy(null);
      loadMyCopies();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleteSubmitting(false);
    }
  }

  async function handleExport() {
    setExportSubmitting(true);
    try {
      await downloadMyCopiesExport(exportFormat);
      setExportOpen(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Export failed");
    } finally {
      setExportSubmitting(false);
    }
  }

  const MAX_IMPORT_FILE_BYTES = 2_000_000; // mirrors the backend's Content maxLength

  function importFormatFromFilename(name: string): MyCopiesExportFormat | null {
    const ext = name.split(".").pop()?.toLowerCase();
    if (ext === "json") return "json";
    if (ext === "yaml" || ext === "yml") return "yaml";
    if (ext === "csv") return "csv";
    return null;
  }

  function resetImportDialog() {
    setImportFile(null);
    setImportPreview(null);
    setImportResult(null);
    setImportError("");
    setImportDecisions({});
    if (importInputRef.current) importInputRef.current.value = "";
  }

  async function handleImportFileSelected(file: File) {
    setImportError("");
    setImportResult(null);
    setImportPreview(null);
    setImportDecisions({});
    const format = importFormatFromFilename(file.name);
    if (!format) {
      setImportError("File must be .json, .yaml, .yml, or .csv");
      return;
    }
    if (file.size > MAX_IMPORT_FILE_BYTES) {
      setImportError("File is too large (max 2MB)");
      return;
    }
    setImportFile(file);
    setImportBusy(true);
    try {
      const content = await file.text();
      const preview = await api.previewImportBooks(format, content);
      setImportPreview(preview);
    } catch (err) {
      setImportError(err instanceof Error ? err.message : "Preview failed");
    } finally {
      setImportBusy(false);
    }
  }

  async function handleImportConfirm() {
    if (!importFile) return;
    const format = importFormatFromFilename(importFile.name);
    if (!format) return;
    setImportBusy(true);
    try {
      const content = await importFile.text();
      const result = await api.importBooks(format, content, importDecisions);
      setImportResult(result);
      setImportPreview(null);
      const added = result.summary.books_created + result.summary.books_matched;
      toast.success(
        added > 0
          ? `Imported ${added} book${added === 1 ? "" : "s"}`
          : "Import finished — nothing new to add",
      );
      loadMyCopies();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Import failed");
    } finally {
      setImportBusy(false);
    }
  }

  async function handleTransfer() {
    if (!transferCopy || !transferEmail.trim()) return;
    setTransferSubmitting(true);
    try {
      await api.transferCopy(transferCopy.id, transferEmail.trim());
      toast.success(`Copy transferred to ${transferEmail}`);
      setTransferCopy(null);
      setTransferEmail("");
      loadMyCopies();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Transfer failed");
    } finally {
      setTransferSubmitting(false);
    }
  }

  const totalCopies = bookGroups.reduce((n, g) => n + g.copies.length, 0);
  const loanedCount = bookGroups.reduce(
    (n, g) => n + g.copies.filter((c) => c.status === "loaned").length,
    0,
  );
  const totalPending = Object.values(pendingCounts).reduce((n, c) => n + c, 0);

  const query = search.trim().toLowerCase();
  const hasActiveFilters =
    !!query ||
    statusFilter !== "all" ||
    conditionFilter !== "all" ||
    sort !== "title";

  function clearFilters() {
    setSearch("");
    setStatusFilter("all");
    setConditionFilter("all");
    setSort("title");
  }

  const filteredGroups = bookGroups
    .filter(
      (g) =>
        !query ||
        g.title.toLowerCase().includes(query) ||
        g.author.toLowerCase().includes(query),
    )
    .map((g) => ({
      ...g,
      copies: g.copies.filter(
        (c) =>
          (statusFilter === "all" || c.status === statusFilter) &&
          (conditionFilter === "all" || c.condition === conditionFilter),
      ),
    }))
    .filter((g) => g.copies.length > 0)
    .sort((a, b) => {
      switch (sort) {
        case "author":
          return (
            a.author.localeCompare(b.author) || a.title.localeCompare(b.title)
          );
        case "copies":
          return (
            b.copies.length - a.copies.length || a.title.localeCompare(b.title)
          );
        case "newest":
          return (
            Math.max(...b.copies.map((c) => c.id)) -
            Math.max(...a.copies.map((c) => c.id))
          );
        case "title":
        default:
          return a.title.localeCompare(b.title);
      }
    });

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-8 w-40" />
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-32 rounded-xl" />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">My Books</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Copies you&apos;ve shared with the community
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => {
              resetImportDialog();
              setImportOpen(true);
            }}
          >
            <Upload className="size-4" />
            Import
          </Button>
          <Button
            variant="outline"
            disabled={totalCopies === 0}
            onClick={() => setExportOpen(true)}
          >
            <Download className="size-4" />
            Export
          </Button>
          <Link href="/share">
            <Button>
              <Plus className="size-4" />
              Share a Book
            </Button>
          </Link>
        </div>
      </div>

      {totalCopies > 0 && (
        <p className="text-sm text-muted-foreground -mt-4">
          {bookGroups.length} {bookGroups.length === 1 ? "book" : "books"} ·{" "}
          {totalCopies} {totalCopies === 1 ? "copy" : "copies"} shared
          {loanedCount > 0 && ` · ${loanedCount} on loan`}
          {totalPending > 0 &&
            ` · ${totalPending} pending request${totalPending === 1 ? "" : "s"}`}
        </p>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      {totalCopies > 0 && (
        <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center">
          <div className="relative flex-1 max-w-xl">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
            <Input
              type="search"
              placeholder="Search your books by title, author…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 h-10"
            />
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-1.5">
              <SlidersHorizontal className="size-4 text-muted-foreground" />
              <Select
                value={sort}
                onValueChange={(v) => setSort(v as typeof sort)}
              >
                <SelectTrigger className="h-10 w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="title">Title A–Z</SelectItem>
                  <SelectItem value="author">Author A–Z</SelectItem>
                  <SelectItem value="copies">Most Copies</SelectItem>
                  <SelectItem value="newest">Recently Added</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="h-10 w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All statuses</SelectItem>
                <SelectItem value="available">Available</SelectItem>
                <SelectItem value="unavailable">Unavailable</SelectItem>
                <SelectItem value="loaned">Loaned</SelectItem>
                <SelectItem value="requested">Requested</SelectItem>
              </SelectContent>
            </Select>

            <Select value={conditionFilter} onValueChange={setConditionFilter}>
              <SelectTrigger className="h-10 w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All conditions</SelectItem>
                <SelectItem value="good">Good</SelectItem>
                <SelectItem value="fair">Fair</SelectItem>
                <SelectItem value="worn">Worn</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}

      {totalCopies > 0 && hasActiveFilters && (
        <div className="flex items-center gap-2 flex-wrap -mt-4">
          {query && (
            <Badge variant="secondary" className="gap-1 pr-1">
              &ldquo;{search.trim()}&rdquo;
              <button
                type="button"
                aria-label="Clear search"
                onClick={() => setSearch("")}
                className="rounded-full hover:bg-background/60 p-0.5"
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
          {statusFilter !== "all" && (
            <Badge variant="secondary" className="gap-1 pr-1 capitalize">
              {statusFilter}
              <button
                type="button"
                aria-label="Remove status filter"
                onClick={() => setStatusFilter("all")}
                className="rounded-full hover:bg-background/60 p-0.5"
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
          {conditionFilter !== "all" && (
            <Badge variant="secondary" className="gap-1 pr-1 capitalize">
              {conditionFilter}
              <button
                type="button"
                aria-label="Remove condition filter"
                onClick={() => setConditionFilter("all")}
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
                onClick={() => setSort("title")}
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

      {totalCopies === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
          <p className="text-muted-foreground">
            You haven&apos;t shared any books yet.
          </p>
          <Link href="/share">
            <Button variant="outline">
              <Plus className="size-4" /> Share your first book
            </Button>
          </Link>
        </div>
      ) : filteredGroups.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-3 py-8 text-center">
          <p className="text-sm text-muted-foreground">
            No books match your filters.
          </p>
          <Button variant="outline" size="sm" onClick={clearFilters}>
            Clear filters
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-6">
          {filteredGroups.map((group) => (
            <div
              key={group.bookId}
              className="rounded-xl border bg-card overflow-hidden"
            >
              {/* Book header */}
              <div className="flex gap-4 p-4 border-b bg-muted/30">
                <Link
                  href={`/catalog/${group.bookId}`}
                  className="w-14 shrink-0 self-start"
                >
                  <div className="relative w-14 aspect-[2/3] rounded overflow-hidden">
                    <BookCover
                      title={group.title}
                      author={group.author}
                      coverUrl={group.coverUrl}
                      alt={group.title}
                      sizes="56px"
                    />
                  </div>
                </Link>
                <div className="min-w-0">
                  <Link
                    href={`/catalog/${group.bookId}`}
                    className="font-semibold text-base hover:underline line-clamp-2"
                  >
                    {group.title}
                  </Link>
                  {group.author && (
                    <p className="text-sm text-muted-foreground mt-0.5">
                      by {group.author}
                    </p>
                  )}
                  <p className="text-xs text-muted-foreground mt-1">
                    {group.copies.length}{" "}
                    {group.copies.length === 1 ? "copy" : "copies"}
                  </p>
                </div>
              </div>

              {/* Copies */}
              <div className="divide-y">
                {group.copies.map((copy) => {
                  const canDelete =
                    copy.status !== "loaned" && copy.status !== "requested";
                  const canTransfer =
                    copy.status !== "loaned" && copy.status !== "requested";
                  const loan =
                    copy.status === "loaned" ? activeLoans[copy.id] : null;

                  return (
                    <div key={copy.id} className="p-4 flex flex-col gap-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge
                          variant={
                            conditionVariant[copy.condition] ?? "outline"
                          }
                          className="capitalize"
                        >
                          {copy.condition}
                        </Badge>
                        <Badge
                          variant={statusVariant[copy.status] ?? "outline"}
                          className="capitalize"
                        >
                          {copy.status}
                        </Badge>
                      </div>

                      {loan &&
                        (() => {
                          const overdue =
                            !!loan.dueDate &&
                            new Date(loan.dueDate) < new Date();
                          return (
                            <p
                              className={cn(
                                "text-xs",
                                overdue
                                  ? "text-destructive font-medium"
                                  : "text-muted-foreground",
                              )}
                            >
                              Loaned to{" "}
                              <span className="font-medium text-foreground">
                                {loan.borrowerName}
                              </span>
                              {loan.dueDate
                                ? ` · ${overdue ? "overdue since" : "due"} ${new Date(loan.dueDate).toLocaleDateString()}`
                                : " · no return date agreed"}
                            </p>
                          );
                        })()}

                      {copy.notes && (
                        <p className="text-xs text-muted-foreground line-clamp-1">
                          {copy.notes}
                        </p>
                      )}

                      <div className="flex flex-wrap items-center gap-2 mt-1">
                        <Link href={`/my-books/${copy.id}/requests`}>
                          <Button size="sm">
                            Manage Requests
                            {!!pendingCounts[copy.id] && (
                              <Badge variant="secondary" className="px-1.5">
                                {pendingCounts[copy.id]}
                              </Badge>
                            )}
                          </Button>
                        </Link>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => openEdit(copy)}
                        >
                          <Pencil className="size-3" /> Edit
                        </Button>
                        {canTransfer && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setTransferCopy(copy);
                              setTransferEmail("");
                            }}
                          >
                            <ArrowRightLeft className="size-3" /> Transfer
                          </Button>
                        )}
                        {canDelete && (
                          <Button
                            size="sm"
                            variant="ghost"
                            className="text-destructive hover:text-destructive hover:bg-destructive/10"
                            onClick={() => setDeleteCopy(copy)}
                          >
                            <Trash2 className="size-3" /> Remove
                          </Button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Edit dialog */}
      <Dialog
        open={!!editCopy}
        onOpenChange={(open) => !open && setEditCopy(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Copy</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label>Condition</Label>
              <RadioGroup
                value={editCondition}
                onValueChange={(v) => setEditCondition(v as Condition)}
                className="flex gap-4"
              >
                {(["good", "fair", "worn"] as Condition[]).map((c) => (
                  <div key={c} className="flex items-center gap-1.5">
                    <RadioGroupItem value={c} id={`condition-${c}`} />
                    <Label
                      htmlFor={`condition-${c}`}
                      className="text-sm font-normal capitalize cursor-pointer"
                    >
                      {c}
                    </Label>
                  </div>
                ))}
              </RadioGroup>
            </div>

            <div className="flex flex-col gap-1.5">
              <Label>Status</Label>
              <RadioGroup
                value={editStatus}
                onValueChange={setEditStatus}
                className="flex flex-wrap items-center gap-4"
                disabled={
                  editCopy?.status === "loaned" ||
                  editCopy?.status === "requested"
                }
              >
                {(["available", "unavailable"] as const).map((s) => (
                  <div key={s} className="flex items-center gap-1.5">
                    <RadioGroupItem value={s} id={`status-${s}`} />
                    <Label
                      htmlFor={`status-${s}`}
                      className="text-sm font-normal capitalize cursor-pointer"
                    >
                      {s}
                    </Label>
                  </div>
                ))}
                {(editCopy?.status === "loaned" ||
                  editCopy?.status === "requested") && (
                  <Badge variant="secondary">
                    {editCopy.status} — cannot change
                  </Badge>
                )}
              </RadioGroup>
            </div>

            <div className="flex flex-col gap-1.5">
              <Label htmlFor="edit-notes">Notes</Label>
              <Textarea
                id="edit-notes"
                className="resize-none"
                value={editNotes}
                onChange={(e) => setEditNotes(e.target.value)}
                placeholder="Any notes about this copy…"
              />
            </div>

            <div className="flex flex-col gap-2.5">
              <div className="flex items-center gap-2">
                <Checkbox
                  id="edit-auto-approve"
                  checked={editAutoApprove}
                  onCheckedChange={(c) => setEditAutoApprove(c === true)}
                />
                <Label
                  htmlFor="edit-auto-approve"
                  className="text-sm font-normal cursor-pointer"
                >
                  Auto-approve if available
                </Label>
              </div>
              <div className="flex items-center gap-2">
                <Checkbox
                  id="edit-return-date-required"
                  checked={editReturnDateRequired}
                  onCheckedChange={(c) => setEditReturnDateRequired(c === true)}
                />
                <Label
                  htmlFor="edit-return-date-required"
                  className="text-sm font-normal cursor-pointer"
                >
                  Require return date from borrower
                </Label>
              </div>
              <div className="flex items-center gap-2">
                <Checkbox
                  id="edit-hide-owner"
                  checked={editHideOwner}
                  onCheckedChange={(c) => setEditHideOwner(c === true)}
                />
                <Label
                  htmlFor="edit-hide-owner"
                  className="text-sm font-normal cursor-pointer"
                >
                  Keep me anonymous (hide my name from borrowers)
                </Label>
              </div>
            </div>
          </div>
          <DialogFooter showCloseButton>
            <Button onClick={handleEditSave} disabled={editSubmitting}>
              {editSubmitting ? "Saving…" : "Save changes"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Transfer dialog */}
      <Dialog
        open={!!transferCopy}
        onOpenChange={(open) => {
          if (!open) {
            setTransferCopy(null);
            setTransferEmail("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Transfer Copy</DialogTitle>
            <DialogDescription>
              Transfer ownership of this copy to another community member. Enter
              their email address below.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="transfer-email">
              Recipient&apos;s email address
            </Label>
            <Input
              id="transfer-email"
              type="email"
              placeholder="member@example.com"
              value={transferEmail}
              onChange={(e) => setTransferEmail(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleTransfer()}
            />
          </div>
          <DialogFooter showCloseButton>
            <Button
              onClick={handleTransfer}
              disabled={transferSubmitting || !transferEmail.trim()}
            >
              {transferSubmitting ? "Transferring…" : "Transfer Copy"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Export dialog */}
      <Dialog open={exportOpen} onOpenChange={setExportOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Export My Books</DialogTitle>
            <DialogDescription>
              Download the books you own as a file, in the format of your
              choice.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label>Format</Label>
            <RadioGroup
              value={exportFormat}
              onValueChange={(v) => setExportFormat(v as MyCopiesExportFormat)}
              className="flex gap-4"
            >
              {(["json", "yaml", "csv"] as MyCopiesExportFormat[]).map((f) => (
                <div key={f} className="flex items-center gap-1.5">
                  <RadioGroupItem value={f} id={`export-format-${f}`} />
                  <Label
                    htmlFor={`export-format-${f}`}
                    className="text-sm font-normal uppercase cursor-pointer"
                  >
                    {f}
                  </Label>
                </div>
              ))}
            </RadioGroup>
          </div>
          <DialogFooter showCloseButton>
            <Button onClick={handleExport} disabled={exportSubmitting}>
              <Download className="size-4" />
              {exportSubmitting ? "Exporting…" : "Export"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Import dialog */}
      <Dialog
        open={importOpen}
        onOpenChange={(open) => {
          setImportOpen(open);
          if (!open) resetImportDialog();
        }}
      >
        <DialogContent className="sm:max-w-xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Import Books</DialogTitle>
            <DialogDescription>
              Upload a JSON, YAML, or CSV file exported from this or another
              bookshelf instance. You&apos;ll see what will happen before
              anything is added.
            </DialogDescription>
          </DialogHeader>

          {!importPreview && !importResult && (
            <div className="flex flex-col gap-2">
              <label
                htmlFor="import-file-input"
                className="flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 text-center text-sm text-muted-foreground cursor-pointer hover:border-foreground/40 hover:bg-muted/40 transition-colors"
              >
                <Upload className="size-6" />
                {importFile ? importFile.name : "Click to choose a file"}
                <span className="text-xs">
                  .json, .yaml, .yml, or .csv — max 2MB
                </span>
              </label>
              <input
                id="import-file-input"
                ref={importInputRef}
                type="file"
                accept=".json,.yaml,.yml,.csv"
                className="sr-only"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleImportFileSelected(file);
                }}
              />
              {importBusy && (
                <p className="text-sm text-muted-foreground">Reading file…</p>
              )}
              {importError && (
                <p className="text-sm text-destructive">{importError}</p>
              )}
            </div>
          )}

          {(importPreview || importResult) && (
            <div className="flex flex-col gap-3">
              <p className="text-sm font-medium">
                {importSummaryText(
                  (importPreview ?? importResult)!.summary,
                  !!importResult,
                )}
              </p>
              <div className="flex flex-col gap-1 max-h-64 overflow-y-auto rounded-md border divide-y">
                {(importPreview ?? importResult)!.rows.map((row) => (
                  <div
                    key={row.row}
                    className="flex flex-col gap-1.5 px-3 py-2 text-sm"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate">
                        {row.title || `Row ${row.row}`}
                      </span>
                      <div className="flex items-center gap-2 shrink-0">
                        {row.action === "skipped" && row.reason && (
                          <span className="text-xs text-muted-foreground">
                            {row.reason}
                          </span>
                        )}
                        <Badge variant={importActionVariant[row.action]}>
                          {importActionLabel[row.action]}
                        </Badge>
                      </div>
                    </div>
                    {row.action === "possible_match" && importPreview && (
                      <div className="flex flex-col gap-1.5 rounded-md bg-muted/40 p-2">
                        <p className="text-xs text-muted-foreground">
                          Matches your existing copy of{" "}
                          <span className="font-medium text-foreground">
                            {row.matched_book_title}
                          </span>
                          {row.matched_book_author
                            ? ` by ${row.matched_book_author}`
                            : ""}
                        </p>
                        <RadioGroup
                          value={importDecisions[row.row] ?? "create_new"}
                          onValueChange={(value) =>
                            setImportDecisions((prev) => ({
                              ...prev,
                              [row.row]: value as ImportDecision,
                            }))
                          }
                          className="flex flex-row gap-4"
                        >
                          <div className="flex items-center gap-1.5">
                            <RadioGroupItem
                              value="create_new"
                              id={`import-decision-${row.row}-new`}
                            />
                            <Label
                              htmlFor={`import-decision-${row.row}-new`}
                              className="text-xs font-normal cursor-pointer"
                            >
                              Add as new
                            </Label>
                          </div>
                          <div className="flex items-center gap-1.5">
                            <RadioGroupItem
                              value="accept_match"
                              id={`import-decision-${row.row}-match`}
                            />
                            <Label
                              htmlFor={`import-decision-${row.row}-match`}
                              className="text-xs font-normal cursor-pointer"
                            >
                              Use existing
                            </Label>
                          </div>
                        </RadioGroup>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          <DialogFooter showCloseButton>
            {importResult ? (
              <Button onClick={() => setImportOpen(false)}>Done</Button>
            ) : importPreview ? (
              <>
                <Button
                  variant="outline"
                  onClick={resetImportDialog}
                  disabled={importBusy}
                >
                  Choose a different file
                </Button>
                <Button onClick={handleImportConfirm} disabled={importBusy}>
                  <Upload className="size-4" />
                  {importBusy
                    ? "Importing…"
                    : `Import ${
                        importPreview.summary.books_created +
                        importPreview.summary.books_matched +
                        importPreview.summary.possible_matches
                      } book${
                        importPreview.summary.books_created +
                          importPreview.summary.books_matched +
                          importPreview.summary.possible_matches ===
                        1
                          ? ""
                          : "s"
                      }`}
                </Button>
              </>
            ) : null}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <Dialog
        open={!!deleteCopy}
        onOpenChange={(open) => !open && setDeleteCopy(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove this copy?</DialogTitle>
            <DialogDescription>
              {deleteCopy?.bookTitle
                ? `This removes your copy of "${deleteCopy.bookTitle}" from the community catalog. This can't be undone.`
                : "This removes the copy from the community catalog. This can't be undone."}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteSubmitting}
            >
              {deleteSubmitting ? "Removing…" : "Remove copy"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
