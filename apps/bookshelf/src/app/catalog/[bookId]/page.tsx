"use client";

import { useEffect, useMemo, useState, useRef } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { toast } from "sonner";
import { ArrowLeft, ChevronRight, Info, RotateCw } from "lucide-react";
import { api } from "@/lib/api";
import type { Book, User, Copy } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CopyCard } from "@/components/CopyCard";
import { BookCover } from "@/components/BookCover";
import { WaitlistButton } from "@/components/WaitlistButton";
import { RecommendButton } from "@/components/RecommendButton";
import { RecommendedBy } from "@/components/RecommendedBy";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

// Default loan window — see apps/bookshelf/docs/return-date-default-spec.md
// for why 30, not a library-style 14. Backend enforces no upper bound; this
// is UX guidance so the borrower doesn't have to guess and the owner has an
// anchor to counter from — see the accept-request counter-date flow in
// loan-request-flow.spec.ts. Tunable code, not a database column — adjust
// this constant (and the backend's matching config) if 30 turns out wrong.
const DEFAULT_LOAN_DAYS = 30;

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

// Clampable description block: keeps long Google Books blurbs from
// pushing the copies section three folds down on mobile. The edition
// footnote — "this description may be from another edition of the book"
// — is a trust signal about accuracy, not decorative, so it's promoted
// from 10px muted italics to an Info popover that the eye actually
// catches.
function BookDescription({
  description,
  descriptionEnriched,
}: {
  description: string;
  descriptionEnriched?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  // Rough heuristic — a 4-line clamp on prose-width fits ~400 chars; if
  // we're under that, no toggle needed at all.
  const isLong = description.length > 400;
  return (
    <div className="flex flex-col gap-1 max-w-prose">
      <p
        className={cn(
          "text-sm text-muted-foreground leading-relaxed whitespace-pre-line",
          isLong && !expanded && "line-clamp-4",
        )}
      >
        {description}
      </p>
      <div className="flex items-center gap-3 text-xs">
        {isLong && (
          <button
            type="button"
            className="text-primary hover:underline"
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? "Show less" : "Show more"}
          </button>
        )}
        {descriptionEnriched && (
          <Popover>
            <PopoverTrigger asChild>
              <button
                type="button"
                className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground"
                aria-label="About this description"
              >
                <Info className="size-3.5" />
                <span>About this description</span>
              </button>
            </PopoverTrigger>
            <PopoverContent className="max-w-xs p-3 text-xs leading-relaxed">
              This description is pulled from another edition of the same book.
              Details like the cover, page count, or introduction may differ
              from the copy you&apos;re actually borrowing.
            </PopoverContent>
          </Popover>
        )}
      </div>
    </div>
  );
}

// Rank copies for display: the user should always land on the copy that
// gives them the fastest path to actually reading the book.
//   0 = available & auto-approve (contact info the moment you tap)
//   1 = available (owner has to approve, but no queue)
//   2 = requested (someone else got there first — join waitlist)
//   3 = loaned (queue behind the current borrower)
//   4 = unavailable (owner has taken it out of circulation)
//   5 = your own copy — pushed to the bottom, it's not actionable here
function copyRank(copy: Copy, currentUserId?: number): number {
  if (currentUserId && copy.owner_id === currentUserId) return 5;
  if (copy.status === "available" && copy.auto_approve) return 0;
  if (copy.status === "available") return 1;
  if (copy.status === "requested") return 2;
  if (copy.status === "loaned") return 3;
  return 4;
}

// Suggested message chips — one-tap common phrasings so the request
// dialog isn't a blank textbox that stalls the flow.
const REQUEST_PROMPTS = [
  "I'll pick it up this weekend",
  "Happy to meet at church on Sunday",
  "Can we arrange a drop-off?",
];

function formatDateInput(d: Date): string {
  return d.toISOString().slice(0, 10);
}

export default function BookDetailPage() {
  const params = useParams();
  const bookId = Number(params.bookId);

  const [book, setBook] = useState<Book | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  // Bumped whenever we want per-copy children (WaitlistButton) to
  // re-read their server state — e.g. after a borrow request accepted
  // one copy, changing the surrounding state.
  const [refreshKey, setRefreshKey] = useState(0);
  // Bumped whenever the recommend toggle resolves, so RecommendedBy
  // re-fetches and reflects the viewer's own change in the facepile.
  const [recommendationsRefreshKey, setRecommendationsRefreshKey] = useState(0);

  // Request dialog state
  const [selectedCopy, setSelectedCopy] = useState<Copy | null>(null);
  const [requestMessage, setRequestMessage] = useState("");
  const [expectedReturnDate, setExpectedReturnDate] = useState("");
  const [requesting, setRequesting] = useState(false);

  // The catalog URL (with page/filter params) that brought the user here,
  // embedded as ?from= by BookCard so the breadcrumb can send them back to
  // the exact page they were on rather than always resetting to /catalog.
  // Validated to /catalog* to guard against open-redirect via a crafted URL.
  const [catalogHref, setCatalogHref] = useState("/catalog");
  useEffect(() => {
    const from = new URLSearchParams(window.location.search).get("from");
    // window.location isn't available during SSR — same setState-in-effect
    // exception as CatalogPage's URL hydration on mount.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (from?.startsWith("/catalog")) setCatalogHref(from);
  }, []);

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

  // Note: on success we clear `error` and bump `refreshKey`; on failure
  // we set `error` but leave `book` intact so a background retry keeps
  // the previous view visible (SWR-style stale-while-revalidate). Only
  // the *initial* load falls back to the skeleton, because there's
  // genuinely nothing to show yet.
  //
  // requestIdRef guards against an older fetch (e.g. a background retry
  // for a book the user has since navigated away from) resolving after a
  // newer one and clobbering it — only the most recently *started* call
  // is allowed to apply its response.
  const requestIdRef = useRef(0);
  function loadBook() {
    const requestId = ++requestIdRef.current;
    return api
      .getBook(bookId)
      .then((b) => {
        if (requestId !== requestIdRef.current) return;
        setBook(b);
        setError("");
        setRefreshKey((k) => k + 1);
      })
      .catch((err) => {
        if (requestId !== requestIdRef.current) return;
        setError(err instanceof Error ? err.message : "Failed to load book");
      })
      .finally(() => {
        if (requestId === requestIdRef.current) setLoading(false);
      });
  }

  useEffect(() => {
    if (!bookId) return;
    void loadBook();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookId]);

  const copies = useMemo(() => book?.copies ?? [], [book?.copies]);
  const sortedCopies = useMemo(
    () =>
      [...copies].sort(
        (a, b) => copyRank(a, currentUser?.id) - copyRank(b, currentUser?.id),
      ),
    [copies, currentUser?.id],
  );

  // The "best copy" the hero CTA acts on: the highest-ranked copy that
  // isn't owned by the current user and is actually available. If none
  // is available, the hero downgrades to a waitlist prompt anchored to
  // the first loaned/requested copy the user can queue on.
  const bestAvailable = useMemo(() => {
    return sortedCopies.find(
      (c) =>
        c.status === "available" &&
        (!currentUser || c.owner_id !== currentUser.id),
    );
  }, [sortedCopies, currentUser]);

  const firstWaitlistable = useMemo(() => {
    return sortedCopies.find(
      (c) =>
        (c.status === "loaned" || c.status === "requested") &&
        (!currentUser || c.owner_id !== currentUser.id),
    );
  }, [sortedCopies, currentUser]);

  function openRequest(copy: Copy) {
    setSelectedCopy(copy);
    setRequestMessage("");
    // Default to +30 days from today so the borrower has a sensible
    // anchor and the owner has a concrete number to counter — the
    // owner-side counter flow is covered by loan-request-flow.spec.ts.
    const d = new Date();
    d.setDate(d.getDate() + DEFAULT_LOAN_DAYS);
    setExpectedReturnDate(formatDateInput(d));
  }

  async function handleRequest() {
    if (!selectedCopy) return;
    if (!expectedReturnDate) {
      toast.error("Return date is required");
      return;
    }
    setRequesting(true);
    try {
      const created = await api.createLoanRequest({
        copy_id: selectedCopy.id,
        message: requestMessage.trim() || undefined,
        expected_return_date: expectedReturnDate,
      });
      toast.success(
        created.status === "accepted"
          ? "Request approved — check Loans for the owner's contact info"
          : "Borrow request sent!",
      );
      setSelectedCopy(null);
      setRequestMessage("");
      setExpectedReturnDate("");
      loadBook();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to send request",
      );
    } finally {
      setRequesting(false);
    }
  }

  if (loading && !book) {
    return (
      <div className="flex flex-col gap-6">
        <Skeleton className="h-6 w-48" />
        <div className="flex flex-row gap-4 sm:gap-6">
          <Skeleton className="w-32 sm:w-48 lg:w-56 aspect-[2/3] rounded-lg shrink-0" />
          <div className="flex flex-col gap-2 flex-1">
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-5 w-24 mt-1" />
          </div>
        </div>
        <Skeleton className="h-11 w-full sm:w-40" />
        <Skeleton className="h-20 w-full max-w-prose" />
      </div>
    );
  }

  if ((error && !book) || !book) {
    // Centered, form-width block — matches the app's other "graceful
    // nothing here" surfaces (login, forgot-password) rather than a
    // flush-left error that reads as "something broke".
    return (
      <div className="mx-auto flex max-w-md flex-col items-center gap-4 py-12 text-center">
        <p className="text-destructive">{error || "Book not found"}</p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button variant="default" onClick={loadBook}>
            <RotateCw className="size-4" /> Try again
          </Button>
          <Button variant="outline" asChild>
            <Link href={catalogHref}>
              <ArrowLeft className="size-4" /> Back to catalog
            </Link>
          </Button>
        </div>
      </div>
    );
  }

  const isOwnerBrowsing = currentUser
    ? copies.some((c) => c.owner_id === currentUser.id)
    : false;
  const ownOnlyCopies =
    copies.length > 0 &&
    currentUser &&
    copies.every((c) => c.owner_id === currentUser.id);
  const primaryLabel = bestAvailable?.auto_approve
    ? "Borrow instantly"
    : "Request to Borrow";

  return (
    <div className="flex flex-col gap-6">
      {/* Breadcrumb works regardless of history state — a fresh link
          from a QR scan or shared URL has no back stack for router.back()
          to fall back to, so we lean on a real anchor to /catalog instead. */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1 text-sm text-muted-foreground -ml-1"
      >
        <Button variant="ghost" size="sm" asChild className="h-8 px-2">
          <Link href={catalogHref} aria-label="Back to catalog">
            <ArrowLeft className="size-4" />
            <span>Catalog</span>
          </Link>
        </Button>
        <ChevronRight className="size-3.5 shrink-0" aria-hidden="true" />
        <span
          className="truncate text-foreground"
          title={book.title}
          aria-current="page"
        >
          {book.title}
        </span>
      </nav>

      {/* Stale-data hint: if a background refetch failed but we still
          have data from the previous fetch, tell the user rather than
          leaving them with an out-of-date view and no recourse. */}
      {error && book && (
        <div className="flex items-center justify-between gap-3 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          <span>Couldn&apos;t refresh — showing the last known state.</span>
          <Button size="sm" variant="outline" onClick={loadBook}>
            <RotateCw className="size-4" /> Retry
          </Button>
        </div>
      )}

      {/* Book header. Layout switches at md:
            - Mobile: album pattern — cover + meta on row 1 (flex-row),
              then CTA on row 2 (full-width thumb target), then
              description on row 3 (full page width).
            - Desktop: two-column grid. Cover pinned left (sticky as
              the user scrolls into the copies list), everything else
              stacks in the right column beside it. The `md:contents`
              trick on the mobile flex-row lets its children (cover +
              meta) participate directly in the outer grid at md+, so
              cover lands in col 1 and meta in col 2 without needing
              duplicate JSX. */}
      <div className="flex flex-col gap-4 md:grid md:grid-cols-[13rem_1fr] md:gap-x-8 md:gap-y-4 lg:grid-cols-[14rem_1fr]">
        <div className="flex flex-row gap-4 md:contents">
          <div className="relative w-32 md:w-52 lg:w-56 aspect-[2/3] rounded-lg overflow-hidden bg-muted shrink-0 self-start md:sticky md:top-6 md:row-span-3">
            <BookCover
              title={book.title}
              author={book.author}
              coverUrl={book.cover_url}
              sizes="(max-width: 640px) 128px, (max-width: 1024px) 208px, 224px"
              fit="contain"
            />
          </div>

          <div className="flex flex-col gap-1.5 min-w-0 flex-1">
            <h1 className="text-xl sm:text-2xl md:text-3xl font-bold leading-tight line-clamp-3">
              {book.title}
            </h1>
            {book.author && (
              <p className="text-muted-foreground text-sm sm:text-base line-clamp-2">
                {book.author}
              </p>
            )}
            {book.isbn && (
              <p className="text-xs text-muted-foreground">ISBN: {book.isbn}</p>
            )}
            {typeof book.available_copies === "number" && (
              <Badge
                variant={book.available_copies > 0 ? "default" : "secondary"}
                className="self-start mt-1"
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
            {/* Same recommend affordance and state as BookCard's, wired to
                the same behavior — tapping either surface toggles the same
                underlying thumbs-up. See docs/book-recommendations-spec.md's
                "Detail-page surface". */}
            <RecommendButton
              bookId={book.id}
              bookTitle={book.title}
              recommended={book.your_recommendation ?? false}
              count={book.recommendation_count ?? 0}
              className="self-start"
              onToggled={() => setRecommendationsRefreshKey((k) => k + 1)}
            />
          </div>
        </div>

        {/* Primary CTA + owner hint. Full-width block on mobile
            (thumb-friendly bar); on desktop it lands in the grid's
            right column (col 2, row 2) directly under the meta block,
            visually beside the cover instead of orphaned below it. */}
        <div className="flex flex-col gap-2 -mt-2 md:mt-0 md:col-start-2">
          {!currentUser ? (
            <Button
              asChild
              size="lg"
              className="w-full sm:w-auto sm:self-start"
            >
              <Link href="/login">Sign in to borrow</Link>
            </Button>
          ) : bestAvailable ? (
            <>
              <Button
                size="lg"
                className="w-full sm:w-auto sm:self-start"
                onClick={() => openRequest(bestAvailable)}
              >
                {primaryLabel}
              </Button>
              <p className="text-xs text-muted-foreground">
                {bestAvailable.hide_owner
                  ? "Shared by an anonymous member"
                  : bestAvailable.owner?.name
                    ? `Shared by ${bestAvailable.owner.name}`
                    : "Shared by a community member"}
                {" · "}
                {bestAvailable.auto_approve
                  ? "Contact info revealed instantly"
                  : "Owner will confirm"}
              </p>
            </>
          ) : firstWaitlistable ? (
            <>
              <Button
                size="lg"
                variant="secondary"
                asChild
                className="w-full sm:w-auto sm:self-start"
              >
                <a href={`#copy-${firstWaitlistable.id}`}>
                  See waitlist options
                </a>
              </Button>
              <p className="text-xs text-muted-foreground">
                No copies free right now — join a waitlist to be notified.
              </p>
            </>
          ) : ownOnlyCopies ? (
            <p className="text-sm text-muted-foreground">
              You&apos;re the only sharer for this book.
            </p>
          ) : (
            <p className="text-sm text-muted-foreground">
              No copies in the library yet.
            </p>
          )}
        </div>

        {book.description && (
          <div className="md:col-start-2">
            <BookDescription
              description={book.description}
              descriptionEnriched={book.description_enriched}
            />
          </div>
        )}
      </div>

      {/* Copies */}
      <div className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold" id="copies">
          {copies.length === 1
            ? "1 copy in the library"
            : `${copies.length} copies in the library`}
        </h2>

        {copies.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No copies in the library yet.
          </p>
        ) : (
          // 2-col grid at lg+ so cards don't stretch to a 1000px+ bar
          // on desktop. Matches the "compact glance cards in a grid"
          // convention documented in apps/bookshelf/CLAUDE.md.
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {sortedCopies.map((copy) => {
              const isOwner = currentUser && copy.owner_id === currentUser.id;
              const isBest = bestAvailable?.id === copy.id;
              const canRequest =
                copy.status === "available" && currentUser && !isOwner;

              const canWaitlist =
                (copy.status === "loaned" || copy.status === "requested") &&
                currentUser &&
                !isOwner;

              const demoted =
                copy.status === "unavailable" || copy.status === "loaned";

              return (
                <div
                  id={`copy-${copy.id}`}
                  key={copy.id}
                  className="scroll-mt-24 h-full"
                >
                  <CopyCard
                    copy={copy}
                    highlighted={isBest}
                    demoted={demoted && !isBest}
                    actions={
                      canRequest ? (
                        // When this is the "best" copy, the hero CTA
                        // above is already the primary action for it —
                        // showing another button here would duplicate
                        // it and confuse strict-mode role queries in
                        // e2e. A subtle marker is enough.
                        isBest ? (
                          <span className="text-xs text-primary font-medium">
                            ↑ Tap the button above to borrow this copy
                          </span>
                        ) : (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => openRequest(copy)}
                          >
                            {copy.auto_approve
                              ? "Borrow instantly"
                              : "Request to Borrow"}
                          </Button>
                        )
                      ) : canWaitlist ? (
                        <WaitlistButton
                          key={`${copy.id}-${refreshKey}`}
                          copyId={copy.id}
                        />
                      ) : isOwner ? (
                        <span className="text-xs text-muted-foreground italic">
                          Your copy
                        </span>
                      ) : !currentUser ? (
                        <Button size="sm" variant="outline" asChild>
                          <Link href="/login">Sign in to borrow</Link>
                        </Button>
                      ) : null
                    }
                  />
                </div>
              );
            })}
          </div>
        )}

        {isOwnerBrowsing && (
          <p className="text-xs text-muted-foreground">
            Manage your copies in{" "}
            <Link href="/my-books" className="text-primary hover:underline">
              My Books
            </Link>
            .
          </p>
        )}

        {/* Renders nothing when nobody has recommended the book — no
            wrapper, no heading — so an untouched book leaves no orphan
            component here, coordinating with ReadingActivityRow's identical
            self-hiding behavior above. See docs/book-recommendations-spec.md's
            "Empty-state coordination with Feature A". */}
        <RecommendedBy
          bookId={book.id}
          refreshKey={recommendationsRefreshKey}
        />
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
            <DialogTitle>
              {selectedCopy?.auto_approve
                ? "Borrow instantly"
                : "Request to Borrow"}
            </DialogTitle>
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
            <Textarea
              id="request-message"
              placeholder="e.g. I'd love to read this for my book club…"
              value={requestMessage}
              onChange={(e) => setRequestMessage(e.target.value)}
            />
            {/* Prompt chips — an empty textbox stalls the flow; giving
                three sensible defaults means the common case is a
                one-tap message rather than a stare at the cursor. */}
            <div className="flex flex-wrap gap-1.5">
              {REQUEST_PROMPTS.map((prompt) => (
                <button
                  key={prompt}
                  type="button"
                  onClick={() => setRequestMessage(prompt)}
                  className="rounded-full border border-input bg-transparent px-2.5 py-1 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  {prompt}
                </button>
              ))}
            </div>
          </div>
          {selectedCopy?.notes && (
            <blockquote className="border-l-2 border-muted-foreground/30 pl-3 text-sm italic text-muted-foreground">
              {selectedCopy.notes}
              {selectedCopy.owner?.name && (
                <footer className="mt-1 text-xs not-italic">
                  — shared by {selectedCopy.owner.name}
                </footer>
              )}
            </blockquote>
          )}
          <div className="flex flex-col gap-2">
            <label htmlFor="return-date" className="text-sm font-medium">
              Expected return date <span className="text-destructive">*</span>
            </label>
            <Input
              id="return-date"
              type="date"
              required
              aria-required="true"
              min={formatDateInput(new Date())}
              value={expectedReturnDate}
              onChange={(e) => setExpectedReturnDate(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              You can propose a different date. The owner can counter this when
              they accept.
            </p>
          </div>
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
