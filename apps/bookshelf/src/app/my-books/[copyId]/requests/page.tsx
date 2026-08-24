"use client";

import { Fragment, useEffect, useState, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, ChevronDown, ChevronRight } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { LoanRequest } from "@/lib/types";
import { ContactReveal } from "@/components/ContactReveal";
import { ReturnDateCell } from "@/components/ReturnDateCell";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type Condition = "good" | "fair" | "worn";

const statusVariant: Record<
  string,
  "success" | "secondary" | "destructive" | "outline"
> = {
  pending: "secondary",
  accepted: "success",
  rejected: "destructive",
  cancelled: "outline",
  returned: "outline",
};

function hasExpandContent(request: LoanRequest): boolean {
  return !!(
    request.message ||
    request.status === "accepted" ||
    request.status === "returned"
  );
}

// expected_return_date comes back as a full RFC3339 timestamp; <input
// type="date"> needs a bare YYYY-MM-DD.
function toDateInputValue(iso?: string): string {
  return iso ? iso.slice(0, 10) : "";
}

export default function CopyRequestsPage() {
  const params = useParams();
  const router = useRouter();

  const copyId = Number(params.copyId);

  const [requests, setRequests] = useState<LoanRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [actioning, setActioning] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  // Accept + return-date dialog
  const [acceptDialog, setAcceptDialog] = useState<{
    requestId: number;
  } | null>(null);
  const [acceptDate, setAcceptDate] = useState("");
  const [acceptProposedDate, setAcceptProposedDate] = useState("");

  // Return + condition dialog
  const [returnDialog, setReturnDialog] = useState<{
    requestId: number;
    currentCondition: string;
  } | null>(null);
  const [returnCondition, setReturnCondition] = useState<Condition>("good");

  // Undo-return confirmation dialog
  const [undoDialog, setUndoDialog] = useState<{ requestId: number } | null>(
    null,
  );
  const [undoing, setUndoing] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) router.push("/login");
  }, [router]);

  const fetchedCopyIdRef = useRef<number | null>(null);
  useEffect(() => {
    if (!copyId || fetchedCopyIdRef.current === copyId) return;
    fetchedCopyIdRef.current = copyId;
    setLoading(true);
    api
      .getLoanRequestsByCopy(copyId)
      .then(setRequests)
      .catch((err) =>
        setError(
          err instanceof Error ? err.message : "Failed to load requests",
        ),
      )
      .finally(() => setLoading(false));
  }, [copyId]);

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  async function handleAction(
    requestId: number,
    status: "accepted" | "rejected" | "returned",
    newCondition?: Condition,
  ) {
    setActioning(requestId);
    try {
      const updated = await api.updateLoanRequest(requestId, {
        status,
        ...(newCondition ? { new_condition: newCondition } : {}),
      });
      setRequests((prev) =>
        prev.map((r) => (r.id === requestId ? updated : r)),
      );
      toast.success(
        status === "accepted"
          ? "Request accepted!"
          : status === "rejected"
            ? "Request declined."
            : "Marked as returned.",
      );
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed");
    } finally {
      setActioning(null);
      setReturnDialog(null);
    }
  }

  function openReturnDialog(requestId: number, currentCondition: string) {
    setReturnCondition((currentCondition as Condition) || "good");
    setReturnDialog({ requestId, currentCondition });
  }

  function openAcceptDialog(request: LoanRequest) {
    const proposed = toDateInputValue(request.expected_return_date);
    setAcceptProposedDate(proposed);
    setAcceptDate(proposed);
    setAcceptDialog({ requestId: request.id });
  }

  async function handleAcceptConfirm() {
    if (!acceptDialog) return;
    const { requestId } = acceptDialog;
    setActioning(requestId);
    try {
      let updated = await api.updateLoanRequest(requestId, {
        status: "accepted",
      });
      if (acceptDate && acceptDate !== acceptProposedDate) {
        updated = await api.updateExpectedReturnDate(requestId, acceptDate);
      }
      setRequests((prev) =>
        prev.map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Request accepted!");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed");
    } finally {
      setActioning(null);
      setAcceptDialog(null);
    }
  }

  async function handleUndoReturn() {
    if (!undoDialog) return;
    setUndoing(true);
    try {
      const updated = await api.updateLoanRequest(undoDialog.requestId, {
        status: "accepted",
      });
      setRequests((prev) =>
        prev.map((r) => (r.id === updated.id ? updated : r)),
      );
      toast.success("Return undone — loan is active again");
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : "Can't undo — this copy has already moved on since it was marked returned.",
      );
    } finally {
      setUndoing(false);
      setUndoDialog(null);
    }
  }

  function handleRequestUpdated(updated: LoanRequest) {
    setRequests((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
  }

  const bookTitle = requests[0]?.copy?.book?.title;
  const bookAuthor = requests[0]?.copy?.book?.author;

  return (
    <div className="flex flex-col gap-6">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => router.push("/my-books")}
        className="self-start -ml-1"
      >
        <ArrowLeft className="size-4" /> Back to My Books
      </Button>

      <div>
        <h1 className="text-2xl font-bold">Manage Requests</h1>
        <p className="text-muted-foreground text-sm mt-1">
          {bookTitle
            ? `${bookTitle}${bookAuthor ? ` · ${bookAuthor}` : ""}`
            : `Copy #${copyId}`}
        </p>
      </div>

      {loading && (
        <div className="flex flex-col gap-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-12" />
          ))}
        </div>
      )}

      {!loading && error && <p className="text-sm text-destructive">{error}</p>}

      {!loading && !error && requests.length === 0 && (
        <div className="rounded-lg border border-dashed p-6 text-center">
          <p className="text-muted-foreground text-sm">
            No requests for this copy yet.
          </p>
        </div>
      )}

      {!loading && requests.length > 0 && (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-8" />
                <TableHead>Requester</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Requested</TableHead>
                <TableHead>Return by</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {requests.map((request) => {
                const expandable = hasExpandContent(request);
                const isExpanded = expanded.has(request.id);

                return (
                  <Fragment key={request.id}>
                    <TableRow
                      onClick={
                        expandable ? () => toggleExpand(request.id) : undefined
                      }
                      className={expandable ? "cursor-pointer" : ""}
                    >
                      <TableCell className="w-8 pr-0">
                        {expandable ? (
                          isExpanded ? (
                            <ChevronDown className="size-4 text-muted-foreground" />
                          ) : (
                            <ChevronRight className="size-4 text-muted-foreground" />
                          )
                        ) : null}
                      </TableCell>
                      <TableCell className="font-medium">
                        {request.borrower?.name ??
                          `User #${request.borrower_id}`}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={statusVariant[request.status] ?? "outline"}
                        >
                          {request.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(request.requested_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <ReturnDateCell
                          request={request}
                          onUpdated={handleRequestUpdated}
                        />
                      </TableCell>
                      <TableCell
                        className="text-right"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <div className="flex gap-2 justify-end">
                          {request.status === "pending" && (
                            <>
                              <Button
                                size="sm"
                                onClick={() => openAcceptDialog(request)}
                                disabled={actioning === request.id}
                              >
                                {actioning === request.id ? "…" : "Accept"}
                              </Button>
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={() =>
                                  handleAction(request.id, "rejected")
                                }
                                disabled={actioning === request.id}
                              >
                                {actioning === request.id ? "…" : "Decline"}
                              </Button>
                            </>
                          )}
                          {request.status === "accepted" && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() =>
                                openReturnDialog(
                                  request.id,
                                  request.copy?.condition ?? "good",
                                )
                              }
                              disabled={actioning === request.id}
                            >
                              {actioning === request.id ? "…" : "Mark Returned"}
                            </Button>
                          )}
                          {request.status === "returned" && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() =>
                                setUndoDialog({ requestId: request.id })
                              }
                            >
                              Undo Return
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>

                    {expandable && isExpanded && (
                      <TableRow
                        key={`${request.id}-detail`}
                        className="hover:bg-transparent"
                      >
                        <TableCell colSpan={6} className="py-0 pb-3 px-8">
                          <div className="flex flex-col gap-3">
                            {request.message && (
                              <div>
                                <p className="text-xs font-medium text-muted-foreground mb-1">
                                  Message
                                </p>
                                <p className="text-sm border rounded-md p-3 bg-muted/50">
                                  {request.message}
                                </p>
                              </div>
                            )}
                            {request.status === "accepted" &&
                              request.borrower && (
                                <div>
                                  <p className="text-xs font-medium text-muted-foreground mb-2">
                                    Borrower contact
                                  </p>
                                  <ContactReveal
                                    name={request.borrower.name}
                                    email={request.borrower.email}
                                    phone={request.borrower.phone}
                                    telegramUsername={
                                      request.borrower.telegram_username
                                    }
                                    whatsappUsername={
                                      request.borrower.whatsapp_username
                                    }
                                    contactNote={request.borrower.contact_note}
                                  />
                                </div>
                              )}
                            {request.status === "returned" && (
                              <div>
                                <p className="text-xs font-medium text-muted-foreground mb-1">
                                  Returned
                                </p>
                                <p className="text-sm text-muted-foreground">
                                  {request.returned_at
                                    ? new Date(
                                        request.returned_at,
                                      ).toLocaleDateString()
                                    : "Return date not recorded"}
                                  {request.returned_by != null &&
                                    (request.returned_by === request.borrower_id
                                      ? " · returned by the borrower"
                                      : " · returned by you")}
                                </p>
                              </div>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                    )}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Accept + return-date dialog */}
      <Dialog
        open={!!acceptDialog}
        onOpenChange={(open) => !open && setAcceptDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Accept Request</DialogTitle>
            <DialogDescription>
              {acceptProposedDate
                ? "The borrower proposed a return date below — change it if you'd rather agree on something else."
                : "Optionally set an agreed return date before accepting."}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <label htmlFor="accept-return-date" className="text-sm font-medium">
              Return by (optional)
            </label>
            <Input
              id="accept-return-date"
              type="date"
              value={acceptDate}
              onChange={(e) => setAcceptDate(e.target.value)}
            />
          </div>
          <DialogFooter showCloseButton>
            <Button onClick={handleAcceptConfirm} disabled={actioning !== null}>
              {actioning !== null ? "Accepting…" : "Accept Request"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Return condition dialog */}
      <Dialog
        open={!!returnDialog}
        onOpenChange={(open) => !open && setReturnDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Mark as Returned</DialogTitle>
            <DialogDescription>
              Record the condition of the book when it was returned.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Condition on return</label>
            <div className="flex gap-4">
              {(["good", "fair", "worn"] as Condition[]).map((c) => (
                <label
                  key={c}
                  className="flex items-center gap-1.5 cursor-pointer"
                >
                  <input
                    type="radio"
                    name="return-condition"
                    value={c}
                    checked={returnCondition === c}
                    onChange={() => setReturnCondition(c)}
                    className="accent-primary"
                  />
                  <span className="text-sm capitalize">{c}</span>
                </label>
              ))}
            </div>
          </div>
          <DialogFooter showCloseButton>
            <Button
              onClick={() =>
                returnDialog &&
                handleAction(
                  returnDialog.requestId,
                  "returned",
                  returnCondition,
                )
              }
              disabled={actioning !== null}
            >
              {actioning !== null ? "Saving…" : "Confirm Return"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Undo-return confirmation dialog */}
      <Dialog
        open={!!undoDialog}
        onOpenChange={(open) => !open && setUndoDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Undo return?</DialogTitle>
            <DialogDescription>
              Not actually returned? This puts the loan back to accepted and
              marks the copy as loaned again.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button onClick={handleUndoReturn} disabled={undoing}>
              {undoing ? "Undoing…" : "Undo Return"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
