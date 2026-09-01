"use client";

import { useEffect, useState, useRef, Fragment } from "react";
import Link from "next/link";
import Image from "next/image";
import { toast } from "sonner";
import { BookOpen, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "@/lib/api";
import type { LoanRequest, PublicContact } from "@/lib/types";
import { BookCover } from "@/components/BookCover";
import { ContactReveal } from "@/components/ContactReveal";
import { CurrentlyBorrowedCard } from "@/components/CurrentlyBorrowedCard";
import { ReturnDateCell } from "@/components/ReturnDateCell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Pagination } from "@/components/ui/Pagination";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SegmentedControl } from "@/components/ui/segmented-control";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;

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

const conditionVariant: Record<string, "default" | "secondary" | "outline"> = {
  good: "default",
  fair: "secondary",
  worn: "outline",
};

function hasExpandContent(req: LoanRequest): boolean {
  return !!(
    req.message ||
    req.status === "accepted" ||
    (req.status === "returned" && req.returned_at)
  );
}

// Per-row derived values shared by the desktop table and the mobile card
// list below — computed once per request, rendered twice (table hidden on
// mobile, cards hidden on desktop, per apps/bookshelf/CLAUDE.md's
// "Cards over dense tables on narrow screens" convention).
interface RowInfo {
  req: LoanRequest;
  bookTitle: string;
  bookAuthor?: string;
  coverUrl?: string;
  copyCondition: string;
  loaner?: PublicContact;
  expandable: boolean;
}

function buildRowInfo(req: LoanRequest): RowInfo {
  return {
    req,
    bookTitle: req.copy?.book?.title ?? `Copy #${req.copy_id}`,
    bookAuthor: req.copy?.book?.author,
    coverUrl: req.copy?.book?.cover_url,
    copyCondition: req.copy?.condition ?? "",
    loaner: req.copy?.owner,
    expandable: hasExpandContent(req),
  };
}

// The message/contact/loan-duration detail shown when a row is expanded —
// identical content in both the table's expand-row and the mobile card, so
// it's factored out once rather than kept in sync in two places.
function ExpandedDetail({
  req,
  loaner,
}: {
  req: LoanRequest;
  loaner?: PublicContact;
}) {
  return (
    <div className="flex flex-col gap-3">
      {req.message && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-1">
            Your message
          </p>
          <p className="text-sm italic text-muted-foreground border rounded-md p-3 bg-muted/50">
            {req.message}
          </p>
        </div>
      )}
      {req.status === "accepted" && loaner && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">
            Loaner contact
          </p>
          <ContactReveal
            name={loaner.name}
            email={loaner.email}
            phone={loaner.phone}
            telegramUsername={loaner.telegram_username}
            whatsappUsername={loaner.whatsapp_username}
            contactNote={loaner.contact_note}
          />
        </div>
      )}
      {req.status === "returned" && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-1">
            Loan duration
          </p>
          <p className="text-sm text-muted-foreground">
            {req.requested_at && req.returned_at
              ? `Borrowed ${new Date(req.requested_at).toLocaleDateString()} → Returned ${new Date(req.returned_at).toLocaleDateString()}`
              : req.returned_at
                ? `Returned ${new Date(req.returned_at).toLocaleDateString()}`
                : "Return date not recorded"}
            {req.returned_by != null &&
              (req.returned_by === req.borrower_id
                ? " · returned by you"
                : " · returned by the owner")}
          </p>
        </div>
      )}
    </div>
  );
}

type RequestsView = "current" | "history";

export function BorrowingTab() {
  const [requests, setRequests] = useState<LoanRequest[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);
  const [cancelling, setCancelling] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [activeLoans, setActiveLoans] = useState<LoanRequest[]>([]);
  const [activeLoading, setActiveLoading] = useState(true);
  const [tab, setTab] = useState<RequestsView>("current");
  const tabMountRef = useRef(true);

  // Return + condition dialog
  const [returnDialog, setReturnDialog] = useState<{
    requestId: number;
    currentCondition: string;
  } | null>(null);
  const [returnCondition, setReturnCondition] = useState<Condition>("good");
  const [returning, setReturning] = useState(false);

  useEffect(() => {
    loadActiveLoans();
    loadRequests(1, tab);
    // tab is intentionally omitted: this effect only handles the initial
    // load, and reading its value at mount time (always "current") is all
    // that's needed — the effect below re-fetches on every later tab change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (tabMountRef.current) {
      tabMountRef.current = false;
      return;
    }
    loadRequests(1, tab);
  }, [tab]);

  async function loadRequests(p: number, view: RequestsView) {
    setLoading(true);
    try {
      const data = await api.getMyLoanRequests({
        page: p,
        page_size: PAGE_SIZE,
        view,
      });
      setRequests(data.items);
      setTotalPages(data.total_pages);
      setPage(p);
    } catch {
      setRequests([]);
    } finally {
      setLoading(false);
    }
  }

  async function loadActiveLoans() {
    setActiveLoading(true);
    try {
      const data = await api.getMyActiveLoans();
      setActiveLoans(data.items);
    } catch {
      setActiveLoans([]);
    } finally {
      setActiveLoading(false);
    }
  }

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

  async function handleCancel(requestId: number) {
    setCancelling(requestId);
    try {
      await api.updateLoanRequest(requestId, { status: "cancelled" });
      await loadRequests(page, tab);
      toast.success("Request cancelled");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel");
    } finally {
      setCancelling(null);
    }
  }

  function openReturnDialog(requestId: number, currentCondition: string) {
    setReturnCondition((currentCondition as Condition) || "good");
    setReturnDialog({ requestId, currentCondition });
  }

  async function handleReturn() {
    if (!returnDialog) return;
    setReturning(true);
    try {
      await api.updateLoanRequest(returnDialog.requestId, {
        status: "returned",
        new_condition: returnCondition,
      });
      await Promise.all([loadRequests(page, tab), loadActiveLoans()]);
      toast.success("Marked as returned");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to return");
    } finally {
      setReturning(false);
      setReturnDialog(null);
    }
  }

  function handleRequestUpdated(updated: LoanRequest) {
    setRequests((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
  }

  function renderActions(req: LoanRequest) {
    if (tab !== "current") return null;
    if (req.status === "pending") {
      return (
        <Button
          variant="outline"
          size="sm"
          onClick={() => handleCancel(req.id)}
          disabled={cancelling === req.id}
        >
          {cancelling === req.id ? "Cancelling…" : "Cancel"}
        </Button>
      );
    }
    if (req.status === "accepted") {
      return (
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            openReturnDialog(req.id, req.copy?.condition ?? "good")
          }
        >
          Mark as Returned
        </Button>
      );
    }
    return null;
  }

  const emptyStateCopy =
    tab === "current" ? "No borrow requests yet." : "No past loans yet.";

  const rows = requests.map(buildRowInfo);

  return (
    <>
      {activeLoading ? (
        <Skeleton className="h-24" />
      ) : (
        activeLoans.length > 0 && (
          <div className="flex flex-col gap-3">
            <h2 className="text-sm font-semibold text-muted-foreground">
              Currently Borrowed ({activeLoans.length})
            </h2>
            <div className="flex flex-col gap-2">
              {activeLoans.map((loan) => (
                <CurrentlyBorrowedCard key={loan.id} request={loan} />
              ))}
            </div>
          </div>
        )
      )}

      <div className="flex flex-col gap-6">
        <div className="flex justify-start md:justify-end">
          <SegmentedControl
            value={tab}
            onValueChange={(v) => setTab(v)}
            aria-label="Filter loans"
            options={[
              { value: "current", label: "Current" },
              { value: "history", label: "History" },
            ]}
          />
        </div>

        <div className="flex flex-col gap-6">
          {loading ? (
            <div className="flex flex-col gap-2">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-12" />
              ))}
            </div>
          ) : requests.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
              <p className="text-muted-foreground">{emptyStateCopy}</p>
              <Link
                href="/catalog"
                className="text-sm text-primary hover:underline"
              >
                Browse the catalog →
              </Link>
            </div>
          ) : (
            <>
              {/* Desktop: dense table. Hidden below md — see the mobile
                  card list alongside it. */}
              <div className="hidden md:block rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-8" />
                      <TableHead>Book</TableHead>
                      <TableHead>Condition</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Requested</TableHead>
                      <TableHead>Return by</TableHead>
                      {tab === "current" && (
                        <TableHead className="text-right">Actions</TableHead>
                      )}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rows.map(
                      ({
                        req,
                        bookTitle,
                        bookAuthor,
                        coverUrl,
                        copyCondition,
                        loaner,
                        expandable,
                      }) => {
                        const isExpanded = expanded.has(req.id);

                        return (
                          <Fragment key={req.id}>
                            <TableRow
                              onClick={
                                expandable
                                  ? () => toggleExpand(req.id)
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
                              <TableCell>
                                <div className="flex items-center gap-3">
                                  <div className="w-12 shrink-0">
                                    {coverUrl ? (
                                      <div className="relative w-12 aspect-[2/3] rounded overflow-hidden">
                                        <Image
                                          src={coverUrl}
                                          alt={bookTitle}
                                          fill
                                          className="object-cover"
                                          sizes="48px"
                                        />
                                      </div>
                                    ) : (
                                      <div className="w-12 aspect-[2/3] rounded bg-muted flex items-center justify-center">
                                        <BookOpen className="size-4 text-muted-foreground" />
                                      </div>
                                    )}
                                  </div>
                                  <div className="min-w-0">
                                    <p className="font-medium truncate max-w-[200px]">
                                      {bookTitle}
                                    </p>
                                    {bookAuthor && (
                                      <p className="text-xs text-muted-foreground truncate max-w-[200px]">
                                        {bookAuthor}
                                      </p>
                                    )}
                                  </div>
                                </div>
                              </TableCell>
                              <TableCell>
                                {copyCondition ? (
                                  <Badge
                                    variant={
                                      conditionVariant[copyCondition] ??
                                      "outline"
                                    }
                                    className="capitalize"
                                  >
                                    {copyCondition}
                                  </Badge>
                                ) : (
                                  "—"
                                )}
                              </TableCell>
                              <TableCell>
                                <Badge
                                  variant={
                                    statusVariant[req.status] ?? "outline"
                                  }
                                >
                                  {req.status}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                {new Date(
                                  req.requested_at,
                                ).toLocaleDateString()}
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                <ReturnDateCell
                                  request={req}
                                  onUpdated={handleRequestUpdated}
                                />
                              </TableCell>
                              {tab === "current" && (
                                <TableCell
                                  className="text-right"
                                  onClick={(e) => e.stopPropagation()}
                                >
                                  {renderActions(req)}
                                </TableCell>
                              )}
                            </TableRow>

                            {expandable && isExpanded && (
                              <TableRow
                                key={`${req.id}-detail`}
                                className="hover:bg-transparent"
                              >
                                <TableCell
                                  colSpan={tab === "current" ? 7 : 6}
                                  className="py-0 pb-3 px-8"
                                >
                                  <ExpandedDetail req={req} loaner={loaner} />
                                </TableCell>
                              </TableRow>
                            )}
                          </Fragment>
                        );
                      },
                    )}
                  </TableBody>
                </Table>
              </div>

              {/* Mobile: one glance card per row, tap to expand detail —
                  same data as the table above, shown below md instead of it. */}
              <div className="flex flex-col gap-3 md:hidden">
                {rows.map(
                  ({
                    req,
                    bookTitle,
                    bookAuthor,
                    coverUrl,
                    copyCondition,
                    loaner,
                    expandable,
                  }) => {
                    const isExpanded = expanded.has(req.id);
                    const actions = renderActions(req);

                    return (
                      <Card key={req.id} className="overflow-hidden py-0 gap-0">
                        <CardContent
                          className={cn(
                            "p-3 flex flex-col gap-3",
                            expandable && "cursor-pointer",
                          )}
                          onClick={
                            expandable ? () => toggleExpand(req.id) : undefined
                          }
                        >
                          <div className="flex gap-3">
                            <div className="relative w-12 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
                              <BookCover
                                title={bookTitle}
                                author={bookAuthor}
                                coverUrl={coverUrl}
                                sizes="48px"
                              />
                            </div>
                            <div className="flex-1 min-w-0 flex flex-col gap-1">
                              <p className="text-sm font-medium leading-snug line-clamp-2">
                                {bookTitle}
                              </p>
                              {bookAuthor && (
                                <p className="text-xs text-muted-foreground line-clamp-1">
                                  {bookAuthor}
                                </p>
                              )}
                              <div className="flex items-center gap-1.5 flex-wrap mt-0.5">
                                <Badge
                                  variant={
                                    statusVariant[req.status] ?? "outline"
                                  }
                                >
                                  {req.status}
                                </Badge>
                                {copyCondition && (
                                  <Badge
                                    variant={
                                      conditionVariant[copyCondition] ??
                                      "outline"
                                    }
                                    className="capitalize"
                                  >
                                    {copyCondition}
                                  </Badge>
                                )}
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
                              {new Date(req.requested_at).toLocaleDateString()}
                            </span>
                            <div onClick={(e) => e.stopPropagation()}>
                              <ReturnDateCell
                                request={req}
                                onUpdated={handleRequestUpdated}
                              />
                            </div>
                          </div>

                          {isExpanded && (
                            <div className="pt-1 border-t">
                              <div className="pt-3">
                                <ExpandedDetail req={req} loaner={loaner} />
                              </div>
                            </div>
                          )}

                          {actions && (
                            <div
                              className="flex"
                              onClick={(e) => e.stopPropagation()}
                            >
                              {actions}
                            </div>
                          )}
                        </CardContent>
                      </Card>
                    );
                  },
                )}
              </div>
            </>
          )}

          {totalPages > 1 && (
            <Pagination
              page={page}
              totalPages={totalPages}
              onPageChange={(p) => loadRequests(p, tab)}
            />
          )}
        </div>
      </div>

      {/* Return condition dialog */}
      <Dialog
        open={!!returnDialog}
        onOpenChange={(open) => !open && setReturnDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Mark as Returned</DialogTitle>
            <DialogDescription>
              Record the condition of the book when you returned it.
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
            <Button onClick={handleReturn} disabled={returning}>
              {returning ? "Saving…" : "Confirm Return"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
