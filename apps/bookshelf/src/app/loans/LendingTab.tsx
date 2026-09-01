"use client";

import { useEffect, useState, useRef, Fragment } from "react";
import Image from "next/image";
import { BookOpen, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "@/lib/api";
import type { LoanRequest, PublicContact } from "@/lib/types";
import { BookCover } from "@/components/BookCover";
import { ContactReveal } from "@/components/ContactReveal";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
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
import { isOverdue } from "@/lib/loanStatus";

const PAGE_SIZE = 20;

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

function hasExpandContent(req: LoanRequest): boolean {
  return !!(
    req.message ||
    req.status === "accepted" ||
    (req.status === "returned" && req.returned_at)
  );
}

// Plain, read-only equivalent of ReturnDateCell — that component exposes an
// inline edit affordance (Popover + api.updateExpectedReturnDate), which
// would make this page a second place to write loan state. Lending is
// read-only by design: actions stay on the per-copy /my-books/{copyId}/requests
// page, this is a display-only overdue treatment matching the same logic.
function returnDateDisplay(req: LoanRequest) {
  const overdue =
    req.status === "accepted" && isOverdue(req.expected_return_date);
  return (
    <div className="flex items-center gap-1">
      <span className={cn(overdue && "text-destructive font-medium")}>
        {req.expected_return_date
          ? new Date(req.expected_return_date).toLocaleDateString()
          : "No return date agreed"}
      </span>
      {overdue && <Badge variant="destructive">Overdue</Badge>}
    </div>
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
  borrower?: PublicContact;
  expandable: boolean;
}

function buildRowInfo(req: LoanRequest): RowInfo {
  return {
    req,
    bookTitle: req.copy?.book?.title ?? `Copy #${req.copy_id}`,
    bookAuthor: req.copy?.book?.author,
    coverUrl: req.copy?.book?.cover_url,
    borrower: req.borrower,
    expandable: hasExpandContent(req),
  };
}

// The message/contact/loan-duration detail shown when a row is expanded —
// identical content in both the table's expand-row and the mobile card, so
// it's factored out once rather than kept in sync in two places.
function ExpandedDetail({
  req,
  borrower,
}: {
  req: LoanRequest;
  borrower?: PublicContact;
}) {
  return (
    <div className="flex flex-col gap-3">
      {req.message && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-1">
            Borrower&apos;s message
          </p>
          <p className="text-sm italic text-muted-foreground border rounded-md p-3 bg-muted/50">
            {req.message}
          </p>
        </div>
      )}
      {req.status === "accepted" && borrower && (
        <div>
          <p className="text-xs font-medium text-muted-foreground mb-2">
            Borrower contact
          </p>
          <ContactReveal
            name={borrower.name}
            email={borrower.email}
            phone={borrower.phone}
            telegramUsername={borrower.telegram_username}
            whatsappUsername={borrower.whatsapp_username}
            contactNote={borrower.contact_note}
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
                ? " · returned by the borrower"
                : " · returned by you")}
          </p>
        </div>
      )}
    </div>
  );
}

type LendingView = "current" | "history";

export function LendingTab() {
  const [requests, setRequests] = useState<LoanRequest[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [tab, setTab] = useState<LendingView>("current");
  const tabMountRef = useRef(true);

  useEffect(() => {
    loadRequests(1, tab);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (tabMountRef.current) {
      tabMountRef.current = false;
      return;
    }
    loadRequests(1, tab);
  }, [tab]);

  async function loadRequests(p: number, view: LendingView) {
    setLoading(true);
    try {
      const data = await api.getMyLendingHistory({
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

  const emptyStateCopy =
    tab === "current"
      ? "No one's borrowing from you yet."
      : "No lending history yet.";

  const rows = requests.map(buildRowInfo);

  return (
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
          </div>
        ) : (
          <>
            {/* Desktop: dense table. Hidden below md — see the mobile card
                list alongside it. */}
            <div className="hidden md:block rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-8" />
                    <TableHead>Book</TableHead>
                    <TableHead>Borrower</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Requested</TableHead>
                    <TableHead>Return by</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map(
                    ({
                      req,
                      bookTitle,
                      bookAuthor,
                      coverUrl,
                      borrower,
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
                                <div className="w-8 shrink-0">
                                  {coverUrl ? (
                                    <div className="relative w-8 aspect-[2/3] rounded overflow-hidden">
                                      <Image
                                        src={coverUrl}
                                        alt={bookTitle}
                                        fill
                                        className="object-cover"
                                        sizes="32px"
                                      />
                                    </div>
                                  ) : (
                                    <div className="w-8 aspect-[2/3] rounded bg-muted flex items-center justify-center">
                                      <BookOpen className="size-3 text-muted-foreground" />
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
                            <TableCell className="font-medium">
                              {borrower?.name ?? `User #${req.borrower_id}`}
                            </TableCell>
                            <TableCell>
                              <Badge
                                variant={statusVariant[req.status] ?? "outline"}
                              >
                                {req.status}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {new Date(req.requested_at).toLocaleDateString()}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {returnDateDisplay(req)}
                            </TableCell>
                          </TableRow>

                          {expandable && isExpanded && (
                            <TableRow
                              key={`${req.id}-detail`}
                              className="hover:bg-transparent"
                            >
                              <TableCell colSpan={6} className="py-0 pb-3 px-8">
                                <ExpandedDetail req={req} borrower={borrower} />
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

            {/* Mobile: one glance card per row, tap to expand detail — same
                data as the table above, shown below md instead of it. */}
            <div className="flex flex-col gap-3 md:hidden">
              {rows.map(
                ({
                  req,
                  bookTitle,
                  bookAuthor,
                  coverUrl,
                  borrower,
                  expandable,
                }) => {
                  const isExpanded = expanded.has(req.id);

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
                            <p className="text-xs text-muted-foreground">
                              to {borrower?.name ?? `User #${req.borrower_id}`}
                            </p>
                            <div className="mt-0.5">
                              <Badge
                                variant={statusVariant[req.status] ?? "outline"}
                              >
                                {req.status}
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
                            {new Date(req.requested_at).toLocaleDateString()}
                          </span>
                          {returnDateDisplay(req)}
                        </div>

                        {isExpanded && (
                          <div className="pt-1 border-t">
                            <div className="pt-3">
                              <ExpandedDetail req={req} borrower={borrower} />
                            </div>
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
  );
}
