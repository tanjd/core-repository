# Book Reviews — spec

**Status:** Approved for build · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `Book`, `Copy`, `LoanRequest`

Let members rate and review books they own or have borrowed, so the catalog carries the
community's own opinion of a title — not just whether a copy is on the shelf.

## Why now

`TODO.md` had parked reviews as _"someday — don't build until there's proven demand."_ Members
have now asked for it directly, alongside a request to see which books are "popular."

The popularity half of that ask is intentionally **not** built here — see Non-goals.

## Goals

- A member who owns or has borrowed a book can leave a 1–5 star rating and an optional comment.
- Anyone browsing a book's page can read existing reviews before requesting to borrow it.
- A review is tied to one person's honest experience — one review per person per book, editable,
  not anonymous.

## Non-goals (v1)

- **Popular Books ranking** — dropped. The existing per-copy Waitlist already shows demand; a
  second ranking surface wasn't worth it for a catalog this size.
- Moderation queue, photo attachments, or upvoting reviews.
- Email notifications for new reviews (in-app only, for now).

## Who can review what

Eligibility is earned, not open — a review should mean the person actually had the book.

- Eligible: the member owns a copy of the book, **or** has an `accepted`/`returned` loan request
  against a copy of it.
- One review per person per book. Submitting again edits the existing review rather than creating
  a second one.
- Only the author can edit or delete their review. No admin override in v1.

## Data model

New `Review` table; one small addition to the existing `Notification` table.

| Field                       | Type     | Notes                                                 |
| --------------------------- | -------- | ----------------------------------------------------- |
| `id`                        | `uint`   | Primary key.                                          |
| `book_id`                   | `uint`   | FK → `books.id`. Part of a unique pair with reviewer. |
| `reviewer_id`               | `uint`   | FK → `users.id`.                                      |
| `rating`                    | `int`    | 1–5.                                                  |
| `comment`                   | `string` | Optional.                                             |
| `created_at` / `updated_at` | `time`   | Standard timestamps.                                  |

Unique index on `(book_id, reviewer_id)` enforces one review per person per book. `Notification`
gains a nullable `review_id` and a new `review_received` type, matching how it already carries
`loan_request_id` / `wishlist_request_id`.

## API surface

| Route                      | Auth        | Behavior                                                                         |
| -------------------------- | ----------- | -------------------------------------------------------------------------------- |
| `GET /books/{id}/reviews`  | Public      | List a book's reviews, newest first, reviewer name only.                         |
| `POST /books/{id}/reviews` | Required    | Create a review. 403 if ineligible; blocked if one already exists for this pair. |
| `PATCH /reviews/{id}`      | Author only | Edit rating/comment.                                                             |
| `DELETE /reviews/{id}`     | Author only | Remove the review.                                                               |

The existing `GET /books` and `GET /books/{id}` responses gain `avg_rating` and `review_count`;
the single-book response additionally carries `you_can_review` and `your_review`, so the
frontend doesn't need a second round trip to know whether to show the write-a-review form.

## Open decisions

**Decision 1 — orphaned book cleanup.** When a book's last copy is removed and it has no
external catalog key, the book itself is deleted (existing behavior). A review has no meaning
once its book is gone, so its reviews are deleted along with it, rather than left pointing at
nothing.

**Decision 2 — who gets notified.** A book can have copies from more than one owner. All of them
are notified when a new review comes in, not just one — a shared title is everyone's business,
and it costs one extra lookup to get right.

## Build order

1. Data model + migration — `Review` table, `Notification.review_id`.
2. Repository layer — eligibility check, aggregate (avg + count), notify-owners lookup. Covered
   by tests alongside the existing repo suite.
3. API handlers + the review-created notification workflow.
4. Frontend — star rating component, reviews list + write form on the book detail page.
5. End-to-end coverage — borrow → return → review → edit/delete, plus the ineligible-user
   rejection, in the real-backend Playwright suite.

## How we'll know it's working

The TODO note this replaces said to build reviews once there's "enough real activity to know if
they matter." Post-launch, that means watching: what share of eligible borrows turn into a
review, whether members check reviews before requesting a copy, and whether review volume
actually differentiates books — if ratings cluster flat at 4–5 stars with no real signal, that's
worth knowing too.

## Implementation notes

The detailed engineering plan (exact files, struct/route signatures, migration numbering,
critical files) lives in this repo's git history via the planning session — see the "Critical
files" and per-file breakdown below when implementation starts.

**Backend** (`apps/bookshelf-backend`):

- `internal/models/models.go`: new `Review{ ID, BookID, ReviewerID, Rating int, Comment string,
CreatedAt, UpdatedAt, Reviewer User }`, unique index on `(book_id, reviewer_id)`. Extend
  `Notification` with `ReviewID *uint` and a new `review_received` type value.
- Migration `000010_create_reviews.{up,down}.sql` (`000009` is the last existing one): `reviews`
  table with the unique constraint above, plus `ALTER TABLE notifications ADD COLUMN review_id
INTEGER REFERENCES reviews(id)`.
- `internal/repository/repository.go` / `gorm/review_repo.go` (new file, modeled on
  `wishlist_repo.go`): `Create`, `GetByID`, `Save`, `Delete`, `FindByBookAndReviewer`,
  `ListByBookID`, `AggregateBatch(bookIDs) (map[uint]ReviewAggregate{AvgRating, ReviewCount},
error)` (batched, same shape as `BookRepository.CountAvailableCopiesBatch`),
  `CanUserReview(bookID, userID) (bool, error)` (ownership-or-completed-loan check — a `Copy`
  row with `OwnerID = userID` for that book, OR an `accepted`/`returned` `LoanRequest` with
  `BorrowerID = userID` joined through a `Copy` of that book — same join shape as
  `admin_repo.go`'s `MostBorrowedBooks` query, scoped to one book+user), `DeleteByBookID(bookID)
error`.
- `internal/repository/gorm/copy_repo.go`: add `ListDistinctOwnerIDsByBookID(bookID uint)
([]uint, error)`.
- `internal/handlers/reviews.go` (new, modeled on `wishlist.go`): the four routes above.
- `internal/handlers/books.go`: `bookResponse` gains `AvgRating *float64`, `ReviewCount int64`
  (via `AggregateBatch`, same pattern as `AvailableCopies`); `getBook` additionally gains
  `YouCanReview bool`, `YourReview *models.Review`.
- `internal/services/review_workflow.go` (new, modeled on `wishlist_workflow.go`):
  `OnReviewCreated(ctx, *Review)` notifies every distinct owner of a copy of that book (skipping
  the reviewer if they're also an owner). Best-effort, log-and-swallow on failure. No email step
  in v1.
- `internal/handlers/copies.go`: `maybeDeleteOrphanedBook` gains a `reviews.DeleteByBookID(bookID)`
  call alongside the existing `wishlists.ClearFulfilledBookID` call, before the book is
  hard-deleted.
- `cmd/server/main.go`: wire `reviewRepo`, `reviewWorkflow`, thread `reviewRepo` into
  `NewBookHandler`/`NewCopyHandler`, construct and register `ReviewHandler`.

**Frontend** (`apps/bookshelf`):

- `src/lib/types.ts`: new `Review` type; extend `Book` with `avg_rating?`, `review_count?`,
  `you_can_review?`, `your_review?` (the last two populated only on the detail response).
- `src/lib/api.ts`: new `// Reviews` section — `getBookReviews`, `createReview`, `updateReview`,
  `deleteReview`.
- `src/components/StarRating.tsx` (new, from scratch — no existing rating primitive in the repo):
  `readOnly` display mode and an interactive 1-5 picker mode, built on plain `lucide-react`
  `Star` icons rather than `radio-group.tsx` (a star picker's hover-preview behavior doesn't map
  cleanly onto Radix radio semantics).
- `src/components/ReviewsSection.tsx` (new): fetches `getBookReviews`, lists reviewer name +
  star rating + comment + date, edit/delete affordance on the current user's own review, and —
  gated on `you_can_review` — a "Write a Review" button opening a `Dialog` form (reusing the same
  `Dialog` pattern as the existing borrow-request dialog).
- `src/app/catalog/[bookId]/page.tsx`: add `<ReviewsSection />` below the existing "Copies"
  section; show a compact read-only `StarRating` next to the existing availability badge.
- Badge/status vocabulary stays untouched — reviews have no status enum, so no new badge colors
  per `apps/bookshelf/CLAUDE.md`'s `success`/`destructive`/`secondary`/`outline` constraint.

### Critical files

- `internal/repository/repository.go`, `internal/repository/gorm/copy_repo.go`
- `internal/handlers/books.go`, `internal/handlers/copies.go`, new `internal/handlers/reviews.go`
- `internal/services/wishlist_workflow.go` (template) → new `internal/services/review_workflow.go`
- `internal/models/models.go`, new migration `internal/db/migrations/000010_create_reviews.*.sql`
- `cmd/server/main.go`
- `src/app/catalog/[bookId]/page.tsx`, `src/lib/api.ts`, `src/lib/types.ts`
- New: `src/components/StarRating.tsx`, `src/components/ReviewsSection.tsx`

### Verification

- Backend: `pnpm nx test bookshelf-backend` and `pnpm nx lint bookshelf-backend` (this app
  enables `gocognit`/`gosec`/`revive` — keep new handler methods small, same split pattern as the
  existing `createLoanRequest`).
- Frontend: exercise manually via `pnpm nx dev bookshelf` against a local backend seeded with a
  few loans (or the existing `screenshot-seed.db`) — confirm review eligibility gating.
- End-to-end: extend `apps/bookshelf-e2e` with the spec in build-order step 5,
  `pnpm nx e2e bookshelf-e2e`.
- Full gate: `pnpm nx affected -t lint test` before merging.
