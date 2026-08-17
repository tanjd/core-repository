"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Image from "next/image";
import { toast } from "sonner";
import { ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import type { Book, User, Copy } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { CopyCard } from "@/components/CopyCard";
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

  useEffect(() => {
    const stored = localStorage.getItem("bookshelf_user");
    if (stored) {
      try {
        setCurrentUser(JSON.parse(stored));
      } catch {
        // ignore
      }
    }
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
      await api.createLoanRequest({
        copy_id: selectedCopy.id,
        message: requestMessage.trim() || undefined,
        expected_return_date: selectedCopy.return_date_required
          ? expectedReturnDate
          : undefined,
      });
      toast.success("Borrow request sent!");
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
        <div className="relative w-36 aspect-[2/3] rounded-lg overflow-hidden bg-muted shrink-0">
          {book.cover_url ? (
            <Image
              src={book.cover_url}
              alt={`Cover of ${book.title}`}
              fill
              className="object-cover"
              sizes="144px"
            />
          ) : (
            <div className="flex h-full items-center justify-center text-muted-foreground text-xs text-center px-2">
              No cover
            </div>
          )}
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
          {book.description && (
            <p className="text-sm text-muted-foreground leading-relaxed max-w-prose mt-2">
              {book.description}
            </p>
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

              const isLoaned = copy.status === "loaned";
              const canWaitlist = isLoaned && currentUser && !isOwner;

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
              Send a borrow request for &quot;{book.title}&quot;. You can
              include an optional message.
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
