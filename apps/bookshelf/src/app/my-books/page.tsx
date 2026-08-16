"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  Plus,
  Pencil,
  Trash2,
  X,
  Check,
  BookOpen,
  ArrowRightLeft,
} from "lucide-react";
import { api } from "@/lib/api";
import type { Copy } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
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

export default function MyBooksPage() {
  const router = useRouter();
  const [bookGroups, setBookGroups] = useState<BookGroup[]>([]);
  const [pendingCounts, setPendingCounts] = useState<Record<number, number>>(
    {},
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

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

  // Delete confirm
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // Fetched separately, after the main copies list renders — a per-copy
  // pending-request count isn't returned by GET /copies/mine, and this
  // shouldn't block the page's primary loading state.
  const loadPendingCounts = useCallback(async (copies: Copy[]) => {
    const results = await Promise.all(
      copies.map((copy) =>
        api
          .getLoanRequestsByCopy(copy.id)
          .then(
            (reqs) =>
              [
                copy.id,
                reqs.filter((r) => r.status === "pending").length,
              ] as const,
          )
          .catch(() => [copy.id, 0] as const),
      ),
    );
    setPendingCounts(Object.fromEntries(results));
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
      loadPendingCounts(copies);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load your books",
      );
    } finally {
      setLoading(false);
    }
  }, [loadPendingCounts]);

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) {
      router.push("/login");
      return;
    }
    const stored = localStorage.getItem("bookshelf_user");
    if (stored) {
      try {
        JSON.parse(stored); // validate JSON
        loadMyCopies();
      } catch {
        router.push("/login");
      }
    } else {
      router.push("/login");
    }
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

  async function handleDelete(copyId: number) {
    try {
      await api.deleteCopy(copyId);
      toast.success("Copy removed");
      setDeletingId(null);
      loadMyCopies();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
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

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="h-8 w-40 rounded bg-muted animate-pulse" />
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-32 rounded-xl bg-muted animate-pulse" />
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
        <Link href="/share">
          <Button>
            <Plus className="size-4" />
            Share a Book
          </Button>
        </Link>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

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
      ) : (
        <div className="flex flex-col gap-6">
          {bookGroups.map((group) => (
            <div
              key={group.bookId}
              className="rounded-xl border bg-card overflow-hidden"
            >
              {/* Book header */}
              <div className="flex gap-4 p-4 border-b bg-muted/30">
                <div className="w-14 shrink-0 self-start">
                  {group.coverUrl ? (
                    <div className="relative w-14 aspect-[2/3] rounded overflow-hidden">
                      <Image
                        src={group.coverUrl}
                        alt={group.title}
                        fill
                        className="object-cover"
                        sizes="56px"
                      />
                    </div>
                  ) : (
                    <div className="w-14 aspect-[2/3] rounded bg-muted flex items-center justify-center">
                      <BookOpen className="size-5 text-muted-foreground" />
                    </div>
                  )}
                </div>
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

                  return (
                    <div key={copy.id} className="p-4 flex flex-col gap-3">
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
                        {copy.notes && (
                          <span className="text-xs text-muted-foreground truncate max-w-[200px]">
                            {copy.notes}
                          </span>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center gap-2">
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
                        {canDelete &&
                          (deletingId === copy.id ? (
                            <div className="flex items-center gap-1">
                              <span className="text-xs text-muted-foreground">
                                Confirm delete?
                              </span>
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={() => handleDelete(copy.id)}
                              >
                                <Check className="size-3" /> Yes
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={() => setDeletingId(null)}
                              >
                                <X className="size-3" />
                              </Button>
                            </div>
                          ) : (
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => setDeletingId(copy.id)}
                            >
                              <Trash2 className="size-3" /> Remove
                            </Button>
                          ))}
                        <Link href={`/my-books/${copy.id}/requests`}>
                          <Button size="sm" variant="secondary">
                            Manage Requests
                            {!!pendingCounts[copy.id] && (
                              <Badge variant="destructive" className="px-1.5">
                                {pendingCounts[copy.id]}
                              </Badge>
                            )}
                          </Button>
                        </Link>
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
              <label className="text-sm font-medium">Condition</label>
              <div className="flex gap-3">
                {(["good", "fair", "worn"] as Condition[]).map((c) => (
                  <label
                    key={c}
                    className="flex items-center gap-1.5 cursor-pointer"
                  >
                    <input
                      type="radio"
                      name="edit-condition"
                      value={c}
                      checked={editCondition === c}
                      onChange={() => setEditCondition(c)}
                      className="accent-primary"
                    />
                    <span className="text-sm capitalize">{c}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Status</label>
              <div className="flex flex-wrap gap-3">
                {(["available", "unavailable"] as const).map((s) => (
                  <label
                    key={s}
                    className="flex items-center gap-1.5 cursor-pointer"
                  >
                    <input
                      type="radio"
                      name="edit-status"
                      value={s}
                      checked={editStatus === s}
                      onChange={() => setEditStatus(s)}
                      className="accent-primary"
                      disabled={
                        editCopy?.status === "loaned" ||
                        editCopy?.status === "requested"
                      }
                    />
                    <span className="text-sm capitalize">{s}</span>
                  </label>
                ))}
                {(editCopy?.status === "loaned" ||
                  editCopy?.status === "requested") && (
                  <Badge variant="secondary" className="self-center">
                    {editCopy.status} — cannot change
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Notes</label>
              <textarea
                className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                value={editNotes}
                onChange={(e) => setEditNotes(e.target.value)}
                placeholder="Any notes about this copy…"
              />
            </div>

            <div className="flex flex-col gap-2">
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={editAutoApprove}
                  onChange={(e) => setEditAutoApprove(e.target.checked)}
                  className="accent-primary"
                />
                <span className="text-sm">Auto-approve if available</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={editReturnDateRequired}
                  onChange={(e) => setEditReturnDateRequired(e.target.checked)}
                  className="accent-primary"
                />
                <span className="text-sm">
                  Require return date from borrower
                </span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={editHideOwner}
                  onChange={(e) => setEditHideOwner(e.target.checked)}
                  className="accent-primary"
                />
                <span className="text-sm">
                  Keep me anonymous (hide my name from borrowers)
                </span>
              </label>
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
            <label htmlFor="transfer-email" className="text-sm font-medium">
              Recipient&apos;s email address
            </label>
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
    </div>
  );
}
