"use client";

import { useCallback, useEffect, useState, useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Plus, Pencil, Trash2, BookOpen, ArrowRightLeft } from "lucide-react";
import { api } from "@/lib/api";
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
        <Link href="/share">
          <Button>
            <Plus className="size-4" />
            Share a Book
          </Button>
        </Link>
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
                <Link
                  href={`/catalog/${group.bookId}`}
                  className="w-14 shrink-0 self-start"
                >
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

                      {loan && (
                        <p className="text-xs text-muted-foreground">
                          Loaned to{" "}
                          <span className="font-medium text-foreground">
                            {loan.borrowerName}
                          </span>
                          {loan.dueDate
                            ? ` · due ${new Date(loan.dueDate).toLocaleDateString()}`
                            : " · no return date agreed"}
                        </p>
                      )}

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
