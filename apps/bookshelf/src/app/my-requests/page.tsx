"use client";

import { useEffect, useState, Fragment } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { toast } from "sonner";
import { BookOpen, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "@/lib/api";
import type { LoanRequest } from "@/lib/types";
import { ContactReveal } from "@/components/ContactReveal";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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

const conditionVariant: Record<string, "default" | "secondary" | "outline"> = {
  good: "default",
  fair: "secondary",
  worn: "outline",
};

function hasExpandContent(req: LoanRequest): boolean {
  return !!(req.message || req.status === "accepted");
}

export default function MyRequestsPage() {
  const router = useRouter();
  const [requests, setRequests] = useState<LoanRequest[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);
  const [cancelling, setCancelling] = useState<number | null>(null);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) {
      router.push("/login");
      return;
    }
    loadRequests(1);
  }, [router]);

  async function loadRequests(p: number) {
    setLoading(true);
    try {
      const data = await api.getMyLoanRequests({
        page: p,
        page_size: PAGE_SIZE,
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

  async function handleCancel(requestId: number) {
    setCancelling(requestId);
    try {
      await api.updateLoanRequest(requestId, { status: "cancelled" });
      await loadRequests(page);
      toast.success("Request cancelled");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel");
    } finally {
      setCancelling(null);
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-8 w-40" />
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-12" />
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-bold">My Requests</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Books you&apos;ve asked to borrow
        </p>
      </div>

      {requests.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <p className="text-muted-foreground">No borrow requests yet.</p>
          <Link
            href="/catalog"
            className="text-sm text-primary hover:underline"
          >
            Browse the catalog →
          </Link>
        </div>
      ) : (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-8" />
                <TableHead>Book</TableHead>
                <TableHead>Condition</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Requested</TableHead>
                <TableHead>Return by</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {requests.map((req) => {
                const bookTitle =
                  req.copy?.book?.title ?? `Copy #${req.copy_id}`;
                const bookAuthor = req.copy?.book?.author;
                const copyCondition = req.copy?.condition ?? "";
                const loaner = req.copy?.owner;
                const coverUrl = req.copy?.book?.cover_url;
                const expandable = hasExpandContent(req);
                const isExpanded = expanded.has(req.id);

                return (
                  <Fragment key={req.id}>
                    <TableRow
                      onClick={
                        expandable ? () => toggleExpand(req.id) : undefined
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
                      <TableCell>
                        {copyCondition ? (
                          <Badge
                            variant={
                              conditionVariant[copyCondition] ?? "outline"
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
                        <Badge variant={statusVariant[req.status] ?? "outline"}>
                          {req.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(req.requested_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {req.expected_return_date
                          ? new Date(
                              req.expected_return_date,
                            ).toLocaleDateString()
                          : "—"}
                      </TableCell>
                      <TableCell
                        className="text-right"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {req.status === "pending" && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleCancel(req.id)}
                            disabled={cancelling === req.id}
                          >
                            {cancelling === req.id ? "Cancelling…" : "Cancel"}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>

                    {expandable && isExpanded && (
                      <TableRow
                        key={`${req.id}-detail`}
                        className="hover:bg-transparent"
                      >
                        <TableCell colSpan={7} className="py-0 pb-3 px-8">
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
                                />
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

      {totalPages > 1 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={loadRequests}
        />
      )}
    </div>
  );
}
