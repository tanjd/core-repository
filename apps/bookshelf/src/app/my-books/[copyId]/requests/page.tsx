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
import { Card, CardContent } from "@/components/ui/card";
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
import { cn } from "@/lib/utils";

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

// The message/contact/returned-info detail shown when a row is expanded —
// identical content in both the table's expand-row and the mobile card, so
// it's factored out once rather than kept in sync in two places.
function ExpandedDetail({ request }: { request: LoanRequest }) {
  return (
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
      {request.status === "accepted" && request.borrower && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">
            Borrower contact
          </p>
          <ContactReveal
            name={request.borrower.name}
            email={request.borrower.email}
            phone={request.borrower.phone}
            telegramUsername={request.borrower.telegram_username}
            whatsappUsername={request.borrower.whatsapp_username}
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
              ? new Date(request.returned_at).toLocaleDateString()
              : "Return date not recorded"}
            {request.returned_by != null &&
              (request.returned_by === request.borrower_id
                ? " · returned by the borrower"
                : " · returned by you")}
          </p>
        </div>
      )}
    </div>
  );
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
    const today = toDateInputValue(new Date().toISOString());
    // A request that sat pending long enough can have a proposed date
    // that's already in the past — don't pre-fill a value the date input's
    // own `min={today}` implies is invalid. Bumping to today (rather than
    // clearing) still lets handleAcceptConfirm's `acceptDate !==
    // acceptProposedDate` check detect the change and PATCH the corrected
    // date.
    const clamped = proposed && proposed < today ? today : proposed;
    setAcceptProposedDate(proposed);
    setAcceptDate(clamped);
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

  // Shared between the desktop table and mobile card list below — same
  // action set either way, per apps/bookshelf/CLAUDE.md's "Cards over dense
  // tables on narrow screens" convention.
  function renderActions(request: LoanRequest) {
    if (request.status === "pending") {
      return (
        <div className="flex gap-2 justify-end">
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
            onClick={() => handleAction(request.id, "rejected")}
            disabled={actioning === request.id}
          >
            {actioning === request.id ? "…" : "Decline"}
          </Button>
        </div>
      );
    }
    if (request.status === "accepted") {
      return (
        <Button
          size="sm"
          variant="outline"
          onClick={() =>
            openReturnDialog(request.id, request.copy?.condition ?? "good")
          }
          disabled={actioning === request.id}
        >
          {actioning === request.id ? "…" : "Mark Returned"}
        </Button>
      );
    }
    if (request.status === "returned") {
      return (
        <Button
          size="sm"
          variant="outline"
          onClick={() => setUndoDialog({ requestId: request.id })}
        >
          Undo Return
        </Button>
      );
    }
    return null;
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
        <>
          {/* Desktop: dense table. Hidden below md — see the mobile card
              list alongside it. */}
          <div className="hidden md:block rounded-md border overflow-x-auto">
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
                          expandable
                            ? () => toggleExpand(request.id)
                            : undefined
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
                          {renderActions(request)}
                        </TableCell>
                      </TableRow>

                      {expandable && isExpanded && (
                        <TableRow
                          key={`${request.id}-detail`}
                          className="hover:bg-transparent"
                        >
                          <TableCell colSpan={6} className="py-0 pb-3 px-8">
                            <ExpandedDetail request={request} />
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })}
              </TableBody>
            </Table>
          </div>

          {/* Mobile: one glance card per requester, tap to expand detail —
              same data as the table above, shown below md instead of it. */}
          <div className="flex flex-col gap-3 md:hidden">
            {requests.map((request) => {
              const expandable = hasExpandContent(request);
              const isExpanded = expanded.has(request.id);
              const actions = renderActions(request);

              return (
                <Card key={request.id} className="overflow-hidden py-0 gap-0">
                  <CardContent
                    className={cn(
                      "p-3 flex flex-col gap-3",
                      expandable && "cursor-pointer",
                    )}
                    onClick={
                      expandable ? () => toggleExpand(request.id) : undefined
                    }
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0 flex flex-col gap-1">
                        <p className="text-sm font-medium leading-snug">
                          {request.borrower?.name ??
                            `User #${request.borrower_id}`}
                        </p>
                        <div className="mt-0.5">
                          <Badge
                            variant={statusVariant[request.status] ?? "outline"}
                          >
                            {request.status}
                          </Badge>
                        </div>
                      </div>
                      {expandable &&
                        (isExpanded ? (
                          <ChevronDown className="size-4 text-muted-foreground shrink-0" />
                        ) : (
                          <ChevronRight className="size-4 text-muted-foreground shrink-0" />
                        ))}
                    </div>

                    <div className="flex flex-col gap-1 text-xs text-muted-foreground">
                      <span>
                        Requested{" "}
                        {new Date(request.requested_at).toLocaleDateString()}
                      </span>
                      <div onClick={(e) => e.stopPropagation()}>
                        <ReturnDateCell
                          request={request}
                          onUpdated={handleRequestUpdated}
                        />
                      </div>
                    </div>

                    {isExpanded && (
                      <div className="pt-1 border-t">
                        <div className="pt-3">
                          <ExpandedDetail request={request} />
                        </div>
                      </div>
                    )}

                    {actions && (
                      <div
                        className="flex justify-end"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {actions}
                      </div>
                    )}
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </>
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
              The borrower proposed the return date below — change it if
              you&apos;d rather agree on something else.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <label htmlFor="accept-return-date" className="text-sm font-medium">
              Return by
            </label>
            <Input
              id="accept-return-date"
              type="date"
              min={toDateInputValue(new Date().toISOString())}
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
