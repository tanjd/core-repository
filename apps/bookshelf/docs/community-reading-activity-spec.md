# Community Reading Activity — spec

**Status:** Approved for build · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `Book`, `Copy`, `LoanRequest` (all existing — no new tables, no migration)

Surface, on each book and across the catalog, how much the community is actually reading a title —
so members can answer _"what's being read here?"_ from data the app already collects, without
building a book-metadata/discovery surface (see `apps/bookshelf-backend/CLAUDE.md`'s "Product
scope").

## Why now

Members asked for two adjacent things: _"which books are people reading?"_ and _"what would people
recommend?"_ This spec answers the first from data we already have — completed loan activity —
without introducing user-generated content, moderation, or per-book opinion surfaces. The second
(topical recommendations) is planned separately in `book-recommendations-spec.md` and gated on this
one shipping and being observed first.

This spec also replaces the earlier `book-reviews-spec.md` (now deprecated), which answered a
different question (per-book star ratings) and conflicted with the product-scope guardrail.

## Goals

- Each book's detail page shows how many completed loans it has in the community and how many
  members are currently on its waitlist.
- The catalog page offers a `Sort by: Popular` option that ranks books by completed loan count.
- Zero new data collection — everything is derived from the existing `LoanRequest`/`Copy`/`Book`
  tables.

## Non-goals (v1)

- **Per-book ratings, reviews, or comments.** Explicitly out of scope — see the deprecated
  `book-reviews-spec.md` and the product-scope note above.
- **Topical/tagged discovery** ("what's good on `prayer`") — planned as Feature B in
  `book-recommendations-spec.md`, deferred until we see whether this feature alone answers the
  ask.
- **Time-windowed popularity** ("popular this month"). All-time count is enough for a catalog this
  size (dozens–low hundreds of books); a rolling window can be added later if the flat count turns
  out to be dominated by long-tail activity.
- **A separate "Popular Books" page or shelf.** The catalog `Sort by: Popular` covers this without
  introducing a second surface to maintain.

## What "completed loan" means

A `LoanRequest` counts toward a book's borrow total when its `status` is `accepted` **or**
`returned` — the same rule the admin dashboard's `MostBorrowedBooks` query already uses
(`internal/repository/gorm/admin_repo.go`, ~line 143). Rejected, cancelled, and pending requests
don't count. This keeps the count aligned with "actual reading activity" rather than "expressed
interest."

Waitlist depth is a separate live number (`WaitlistEntry` rows for the book's copies) — shown
alongside the borrow count but not folded into it.

## Data surface

No schema changes. Two new response fields on the existing book endpoints, computed via batched
lookups (same pattern as `AvailableCopies`):

| Field            | Type  | Where                           | Source                                                                   |
| ---------------- | ----- | ------------------------------- | ------------------------------------------------------------------------ |
| `borrow_count`   | `int` | `GET /books`, `GET /books/{id}` | `COUNT(*)` on `loan_requests` where `status IN ('accepted','returned')`. |
| `waitlist_count` | `int` | `GET /books`, `GET /books/{id}` | `COUNT(*)` on `waitlist_entries` joined through `copies` of that book.   |

The single-book response gets both fields the same way — no extra endpoint. Rendering waitlist
depth on the list view is optional (see Frontend below) but the field is returned regardless, so
the frontend can decide without a second round trip.

## API surface

No new routes. One new query parameter and two new response fields on existing routes:

| Route               | Change                                                                                   |
| ------------------- | ---------------------------------------------------------------------------------------- |
| `GET /books`        | `sort` accepts a new value `popular` (borrow_count DESC, then title ASC as tiebreaker).  |
| `GET /books`        | Each item gains `borrow_count` and `waitlist_count`.                                     |
| `GET /books/{id}`   | Response gains `borrow_count` and `waitlist_count`.                                      |
| `GET /books/recent` | Each item gains `borrow_count` and `waitlist_count` (for consistency; not sorted by it). |

`sort=popular` behaves the same as any other sort in `buildListQuery` — it pairs with `q`,
`available_only`, and pagination unchanged. When `q` is set with `sort=popular`, popularity wins
over relevance (matching the "sort by X" mental model — the user picked popularity explicitly).

## Build order

1. **Repository layer** — add `CountBorrowsBatch(bookIDs []uint) (map[uint]int64, error)` and
   `CountWaitlistBatch(bookIDs []uint) (map[uint]int64, error)` to `BookRepository`, modeled
   verbatim on `CountAvailableCopiesBatch` (`internal/repository/gorm/book_repo.go` ~line 180).
   Extend `buildListQuery`'s `switch sort` with a `case "popular"` that `LEFT JOIN`s a subquery
   `(SELECT copies.book_id, COUNT(*) AS borrow_count FROM loan_requests JOIN copies ON
copies.id = loan_requests.copy_id WHERE loan_requests.status IN ('accepted','returned') GROUP BY
copies.book_id)` and orders by `COALESCE(borrow_count, 0) DESC, title ASC`. Repo tests alongside
   the existing `book_repo_test.go` cover: empty catalog, zero-borrow book, tied counts (title
   tiebreaker), interaction with `available_only`.
2. **Handler layer** — extend `bookResponse` (`internal/handlers/books.go`) with `BorrowCount
int64` and `WaitlistCount int64`. Update `toBookResponse` and `toBooksResponse` to populate both
   (batch in the list path, single-shot in the detail path via new `CountBorrows(bookID)` /
   `CountWaitlist(bookID)` singletons — same shape as `CountAvailableCopies` /
   `CountAvailableCopiesBatch` already pair). Handler tests in `books_test.go` cover the new
   fields' presence and the `sort=popular` ordering.
3. **Frontend types + API** — `src/lib/types.ts`: `Book` gains `borrow_count?: number` and
   `waitlist_count?: number`. `src/lib/api.ts`: `getBooks`'s `sort` param accepts `"popular"` (its
   type is already `string`, so this is a documentation change; add it to `SORT_LABELS`).
4. **Frontend UI** —
   - `src/app/catalog/page.tsx`: extend `SORT_LABELS` with `popular: "Most Borrowed"`. It sits
     alongside Title/Author/Newest/Best Match — no new UI surface, just an extra option in the
     existing `<Select>`.
   - `src/app/catalog/[bookId]/page.tsx`: below the availability `Badge` (around line 162),
     render a small stats row: _"Borrowed N times · M on waitlist"_ (omit each half when its count
     is 0 — if both are 0, hide the row entirely rather than showing "Borrowed 0 times"). Plain
     text, no new badge variants — the app's badge vocabulary is fixed
     (`success`/`destructive`/`secondary`/`outline`, per `apps/bookshelf/CLAUDE.md`).
   - `src/components/BookCard.tsx`: **do not** add borrow/waitlist counts to the card. The card
     is a scannable grid tile; the count is a detail-page-only concept for v1. (Revisit if
     `sort=popular` usage is high but detail-page clickthrough is not.)
5. **End-to-end coverage** — one `apps/bookshelf-e2e` spec: seed two books, complete a loan on
   one, verify (a) the detail page shows "Borrowed 1 time", (b) `sort=popular` on the catalog
   ranks it above the un-borrowed book. Extend `screenshot-seed.db` if the existing seed doesn't
   already cover a completed loan.

## Critical files

- `internal/repository/repository.go`, `internal/repository/gorm/book_repo.go`,
  `internal/repository/gorm/book_repo_test.go`
- `internal/handlers/books.go`, `internal/handlers/books_test.go`
- `src/lib/types.ts`, `src/lib/api.ts`
- `src/app/catalog/page.tsx`, `src/app/catalog/[bookId]/page.tsx`
- New e2e spec in `apps/bookshelf-e2e/tests/`

## Verification

- Backend: `pnpm nx test bookshelf-backend` and `pnpm nx lint bookshelf-backend`.
- Frontend: `pnpm nx dev bookshelf` against a local backend with `screenshot-seed.db`, confirm
  `sort=popular` and the detail-page stats row render correctly.
- End-to-end: `pnpm nx e2e bookshelf-e2e`.
- Full gate: `pnpm nx affected -t lint test` before merging.

## How we'll know it's working

- The `sort=popular` option in the catalog gets used (measurable via server logs — the query
  string is already logged by `middleware/logging.go`).
- Detail-page clickthrough on high-borrow books is higher than on low-borrow books (weak signal,
  but the direction is what matters).
- Whether members follow up asking for the topical/tagged version (Feature B) — if they don't,
  Feature B stays deferred indefinitely, and this feature was sufficient.

If the "popular" sort is never used within a month, hide the option (don't remove the fields — the
detail-page stats row is still useful on its own).
