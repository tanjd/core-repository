"use client";

import { useEffect, useState, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import { ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import type { Book, User, Copy } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { CopyCard } from "@/components/CopyCard";
import { BookCover } from "@/components/BookCover";
import { WaitlistButton } from "@/components/WaitlistButton";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

// ReadingActivityRow renders the "Borrowed N times · M on waitlist" strip
// below the availability badge. Each half is omitted when its count is 0
// (and the whole row disappears when both are 0) so an untouched book stays
// visually quiet — see docs/community-reading-activity-spec.md.
function ReadingActivityRow({
  borrowCount,
  waitlistCount,
}: {
  borrowCount?: number;
  waitlistCount?: number;
}) {
  const borrows = borrowCount ?? 0;
  const waiters = waitlistCount ?? 0;
  if (borrows === 0 && waiters === 0) return null;
  const parts: string[] = [];
  if (borrows > 0) {
    parts.push(`Borrowed ${borrows} time${borrows === 1 ? "" : "s"}`);
  }
  if (waiters > 0) {
    parts.push(`${waiters} on waitlist`);
  }
  return <p className="text-xs text-muted-foreground">{parts.join(" · ")}</p>;
}

export default function BookDetailPage() {
  const params = useParams();
  const router = useRouter();
  const bookId = Number(params.bookId);

  const [book, setBook] = useState<Book | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [currentUser, setCurrentUser] = useState<User | null>(null);

  // Request dialog state
  const [selectedCopy, setSelectedCopy] = useState<Copy | null>(null);
  const [requestMessage, setRequestMessage] = useState("");
  const [expectedReturnDate, setExpectedReturnDate] = useState("");
  const [requesting, setRequesting] = useState(false);

  const identifiedRef = useRef(false);
  useEffect(() => {
    if (identifiedRef.current) return;
    identifiedRef.current = true;
    const stored = localStorage.getItem("bookshelf_user");
    let user: User | null = null;
    if (stored) {
      try {
        user = JSON.parse(stored);
      } catch {
        // ignore
      }
    }
    setCurrentUser(user);
  }, []);

  useEffect(() => {
    if (!bookId) return;
    api
      .getBook(bookId)
      .then(setBook)
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Failed to load book"),
      )
      .finally(() => setLoading(false));
  }, [bookId]);

  async function handleRequest() {
    if (!selectedCopy) return;
    if (selectedCopy.return_date_required && !expectedReturnDate) {
      toast.error("Return date is required by the sharer");
      return;
    }
    setRequesting(true);
    try {
      const created = await api.createLoanRequest({
        copy_id: selectedCopy.id,
        message: requestMessage.trim() || undefined,
        expected_return_date: selectedCopy.return_date_required
          ? expectedReturnDate
          : undefined,
      });
      toast.success(
        created.status === "accepted"
          ? "Request approved — check My Requests for the owner's contact info"
          : "Borrow request sent!",
      );
      setSelectedCopy(null);
      setRequestMessage("");
      setExpectedReturnDate("");
      // Refresh book to update copy status
      const updated = await api.getBook(bookId);
      setBook(updated);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to send request",
      );
    } finally {
      setRequesting(false);
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-6">
        <Skeleton className="h-8 w-48" />
        <div className="flex gap-6">
          <Skeleton className="w-40 aspect-[2/3] rounded-lg shrink-0" />
          <div className="flex flex-col gap-3 flex-1">
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-24" />
          </div>
        </div>
      </div>
    );
  }

  if (error || !book) {
    return (
      <div className="flex flex-col gap-4">
        <p className="text-destructive">{error || "Book not found"}</p>
        <Button variant="outline" onClick={() => router.back()}>
          <ArrowLeft className="size-4" /> Go back
        </Button>
      </div>
    );
  }

  const copies = book.copies ?? [];

  return (
    <div className="flex flex-col gap-6">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => router.back()}
        className="self-start -ml-1"
      >
        <ArrowLeft className="size-4" /> Back
      </Button>

      {/* Book header */}
      <div className="flex flex-col sm:flex-row gap-6">
        <div className="relative w-36 sm:w-48 lg:w-56 aspect-[2/3] rounded-lg overflow-hidden bg-muted shrink-0">
          <BookCover
            title={book.title}
            author={book.author}
            coverUrl={book.cover_url}
            sizes="(max-width: 640px) 144px, (max-width: 1024px) 192px, 224px"
          />
        </div>

        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-bold leading-tight">{book.title}</h1>
          {book.author && (
            <p className="text-muted-foreground">{book.author}</p>
          )}
          {book.isbn && (
            <p className="text-xs text-muted-foreground">ISBN: {book.isbn}</p>
          )}
          {typeof book.available_copies === "number" && (
            <Badge
              variant={book.available_copies > 0 ? "default" : "secondary"}
              className="self-start"
            >
              {book.available_copies > 0
                ? `${book.available_copies} copy available`
                : "No copies available"}
            </Badge>
          )}
          <ReadingActivityRow
            borrowCount={book.borrow_count}
            waitlistCount={book.waitlist_count}
          />
          {book.description && (
            <div className="flex flex-col gap-1 mt-2">
              <p className="text-sm text-muted-foreground leading-relaxed max-w-prose">
                {book.description}
              </p>
              {book.description_enriched && (
                <p className="text-[10px] text-muted-foreground italic">
                  Description from another edition
                </p>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Copies */}
      <div className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold">Copies ({copies.length})</h2>

        {copies.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No copies in the library yet.
          </p>
        ) : (
          <div className="flex flex-col gap-3">
            {copies.map((copy) => {
              const isOwner = currentUser && copy.owner_id === currentUser.id;
              const canRequest =
                copy.status === "available" && currentUser && !isOwner;

              const canWaitlist =
                (copy.status === "loaned" || copy.status === "requested") &&
                currentUser &&
                !isOwner;

              return (
                <CopyCard
                  key={copy.id}
                  copy={copy}
                  actions={
                    canRequest ? (
                      <Button size="sm" onClick={() => setSelectedCopy(copy)}>
                        Request to Borrow
                      </Button>
                    ) : canWaitlist ? (
                      <WaitlistButton copyId={copy.id} />
                    ) : isOwner ? (
                      <span className="text-xs text-muted-foreground italic">
                        Your copy
                      </span>
                    ) : null
                  }
                />
              );
            })}
          </div>
        )}
      </div>

      {/* Request dialog */}
      <Dialog
        open={!!selectedCopy}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedCopy(null);
            setExpectedReturnDate("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Request to Borrow</DialogTitle>
            <DialogDescription>
              {selectedCopy?.auto_approve
                ? `This copy auto-approves — you'll get the owner's contact info right away. You can include an optional message.`
                : `Send a borrow request for "${book.title}". You can include an optional message.`}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <label htmlFor="request-message" className="text-sm font-medium">
              Message (optional)
            </label>
            <textarea
              id="request-message"
              className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              placeholder="e.g. I'd love to read this for my book club…"
              value={requestMessage}
              onChange={(e) => setRequestMessage(e.target.value)}
            />
          </div>
          {selectedCopy?.return_date_required && (
            <div className="flex flex-col gap-2">
              <label htmlFor="return-date" className="text-sm font-medium">
                Expected return date <span className="text-destructive">*</span>
              </label>
              <Input
                id="return-date"
                type="date"
                required
                value={expectedReturnDate}
                onChange={(e) => setExpectedReturnDate(e.target.value)}
              />
            </div>
          )}
          <DialogFooter showCloseButton>
            <Button onClick={handleRequest} disabled={requesting}>
              {requesting ? "Sending…" : "Send Request"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
