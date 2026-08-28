# Book Recommendations — implementation plan

Companion to `book-recommendations-spec.md`. The spec is the behavior contract ("what and why").
This plan is the wiring against this specific codebase ("how"). If requirements shift, the spec
changes; if the codebase shifts, this plan changes.

**Prerequisite:** the spec's `Status:` header is Approved for build.

## Auth pattern this feature follows

As of this feature landing, `GET /books`, `GET /books/{id}`, and `GET /books/recent` are
auth-required at the API layer, not just at the frontend — matching `POST /books`:

- Every book route (read and write) declares `Security: []map[string][]string{{"bearer": {}}}`
  on the huma operation _and_ calls `middleware.GetRequiredUserID(ctx)` at the top of the
  handler, returning `huma.Error401Unauthorized` when absent. `getBook`'s `canSeeOwner` logic
  (owner names revealed only to authenticated + email-verified callers) still applies on top of
  that — verification is a separate, stricter gate than plain authentication.
- The frontend's `apps/bookshelf/src/app/catalog/layout.tsx` `<AuthGuard>` wrap is now a
  belt-and-suspenders UX nicety (redirect before a round-trip), not the only enforcement — the
  backend no longer trusts it. Anything that read the catalog unauthenticated (OpenAPI docs
  tooling, ad hoc scripts) needs a session now; `generate-landing-screenshots.ts` already logs in
  as admin first, so it's unaffected. See `apps/bookshelf-backend/internal/handlers/books_test.go`'s
  `TestListBooks_Unauthenticated`/`TestListRecentBooks_Unauthenticated`/`TestGetBook_Unauthenticated`
  for the contract.

For this feature:

- `POST` and `DELETE /books/{id}/recommendations` — auth-required (`Security: bearer` +
  `GetRequiredUserID`), same as always.
- `GET /books/{id}/recommendations` — still public; this one wasn't revisited when the catalog
  reads above were locked down. It's a narrower surface (one book's recommender names, no
  loan/copy data) and nothing currently depends on it staying public — flag for a follow-up
  decision if the auth-required convention should extend to it too, rather than assuming either
  way here.
- `your_recommendation` on `GET /books` and `GET /books/{id}` — reads
  `middleware.GetRequiredUserID(ctx)` (the request already 401s before this point if absent), so
  in practice it's always populated for the caller — the `false`-default code path only matters
  for `toBooksResponse`'s nil-`recommendations`-repo guard, not for anonymity anymore.
- `recommendation_count` — always populated, regardless of session.

## Design decisions the spec deliberately left open

### 1. Deleted-member cleanup: cascade at delete time

Spec requires that deleted members' thumbs-ups fall out of counts and facepiles. Two options
were on the table:

- **Cascade at delete time.** `deleteUser` in `internal/handlers/admin.go` gets one new line
  that clears the user's `Recommendation` rows before `admin.DeleteUser`. Matches the existing
  `wishlists.ClearFulfilledBookID` pattern in `maybeDeleteOrphanedBook`. One-shot cost at
  delete; every subsequent read is trivial.
- **Filter at read.** Every count/list query joins to `users` and filters. Adds cost to every
  hot path. Also requires deciding what "still exists" means when this app hard-deletes users
  (no soft-delete flag), which means the filter is effectively `EXISTS (SELECT 1 FROM users
WHERE id = recommender_id)` on every read.

**Going with cascade.** Cheaper at read (which is 99%+ of traffic), matches the existing
`wishlists.ClearFulfilledBookID`-in-`maybeDeleteOrphanedBook` idiom, and doesn't touch every
query in the read path. Same treatment applies for `maybeDeleteOrphanedBook` on the book side.

### 2. `your_recommendation` on list responses: batch query

`GET /books` needs `your_recommendation` per item so cards can render toggle state (spec Goals).
Batch it, same shape as `CountAvailableCopiesBatch`:

- `HasRecommendedBatch(userID uint, bookIDs []uint) (map[uint]bool, error)` on the recommendation
  repo. One `SELECT book_id FROM recommendations WHERE recommender_id = ? AND book_id IN (...)`.
- Called once per list request from `toBooksResponse`. Skipped entirely for unauthenticated
  requests — the field defaults to `false` and no query runs.

### 3. Race handling on concurrent POST

Two rapid `POST`s from the same member on the same book: the unique constraint on
`(book_id, recommender_id)` catches the second one. Handler must treat the unique-violation
error as the idempotent-success path, not a 500.

### 4. HTTP status codes

- `POST /books/{id}/recommendations` — 201 on new row, 200 on already-exists.
- `DELETE /books/{id}/recommendations` — 204 always (present or absent).
- `GET /books/{id}/recommendations` — 200 with an array (possibly empty).

### 5. Sort param

`sort=recommended` — mirrors `sort=popular`. `LEFT JOIN` a `(book_id, count(*))` subquery on
`recommendations`, order by `COALESCE(count, 0) DESC, title ASC` (title tiebreaker matches
Feature A's convention). Interacts with `q`, `available_only`, and pagination the same way
`sort=popular` does — popularity wins over relevance when both `q` and `sort=recommended` are
set.

### 6. `InitialsAvatar` extracted up front

`CopyCard.tsx`'s inline `OwnerAvatar` (a `size-7 rounded-full bg-muted` initials circle) is
lifted to `src/components/InitialsAvatar.tsx` as the first frontend commit. `CopyCard.tsx`
switches to it in the same commit. Everything downstream (`RecommendedBy`, the facepile) uses
the shared component. No behavior change to copy cards.

## Data shape

- Table `recommendations`: `id`, `book_id`, `recommender_id`, `created_at`. No `updated_at`.
- Unique index on `(book_id, recommender_id)`. That composite index also covers `book_id`-only
  lookups via leftmost-prefix — no separate `book_id` index needed. No `recommender_id`-only
  index in v1 (no user-scoped listing endpoint yet).
- New `Recommendation` struct in `internal/models/models.go`, GORM tags matching the app's
  existing pattern.
- Next migration number is `000015_create_recommendations.{up,down}.sql` — check
  `internal/db/migrations/` at start time in case new migrations land before this feature
  begins.

## Endpoint shape

| Route                                | Auth     | Behavior                                                                                                         |
| ------------------------------------ | -------- | ---------------------------------------------------------------------------------------------------------------- |
| `POST /books/{id}/recommendations`   | Required | Add current member's thumbs-up. 201 new / 200 existing.                                                          |
| `DELETE /books/{id}/recommendations` | Required | Remove current member's own thumbs-up. 204 always.                                                               |
| `GET /books/{id}/recommendations`    | Public   | List recommenders newest first as `{recommender_name, created_at}[]`. No pagination — bounded by community size. |

`GET /books` and `GET /books/{id}` gain `recommendation_count` (int64) and `your_recommendation`
(bool). `your_recommendation` follows the `canSeeOwner` pattern — falls back to `false` when
the session is absent, mirroring how these endpoints already handle owner-name enrichment.

Register the three new routes in `RecommendationHandler.RegisterRoutes` before any `/books/{id}`
wildcard (see `apps/bookshelf-backend/CLAUDE.md`'s wishlist route-ordering note — huma wildcards
swallow literal paths).

## Test cases (write these first, per the TDD rule)

### Repo layer (`recommendation_repo_test.go`)

- `Create` succeeds for a new (book, recommender) pair.
- `Create` returns a unique-violation error for the same pair twice.
- `Delete` succeeds for an existing row; is idempotent (no error) for an absent one.
- `FindByBookAndRecommender` returns the row or `repository.ErrNotFound`.
- `ListByBookID` orders newest-first and preloads the `Recommender` association.
- `CountByBookBatch` returns 0 (or omits — pick one and be consistent) for books with no rows,
  correct counts for the rest.
- `HasRecommendedBatch(userID, bookIDs)` returns `true` only for books the user has actually
  recommended; absent book IDs default to `false`.
- `DeleteByBookID` removes all rows for that book, no error for a book with no rows.
- `DeleteByRecommenderID` removes all rows for that user, no error for a user with no rows.

### Handler layer (`recommendations_test.go`, `books_test.go` additions)

- `POST` unauthenticated → 401.
- `POST` new → 201, `bookResponse.recommendation_count` increments, `your_recommendation` = true.
- `POST` existing → 200, no duplicate row created.
- Concurrent-race path: simulated unique-violation on `Create` returns the idempotent success,
  not 500.
- `DELETE` unauthenticated → 401.
- `DELETE` existing → 204, count decrements.
- `DELETE` absent → 204 (idempotent).
- `GET /books/{id}/recommendations` returns all recommenders newest-first, with
  `recommender_name` populated.
- `GET /books/{id}` includes `recommendation_count` and `your_recommendation` for an
  authenticated viewer; `your_recommendation` falls back to `false` without a session (mirrors
  the `canSeeOwner` degradation).
- `GET /books` includes both fields per item; `your_recommendation` correctly reflects the
  current viewer's state per book, and defaults to `false` on every item without a session.
- `sort=recommended` orders by count DESC then title ASC (with tied counts).
- `sort=recommended` interacts correctly with `q`, `available_only`, and pagination.

### Cascade coverage (`admin_test.go`, `copies_test.go` additions)

- `deleteUser` removes the target's `Recommendation` rows before deleting the user; their
  recommendations no longer contribute to any book's count or facepile.
- `maybeDeleteOrphanedBook` removes the orphaned book's `Recommendation` rows.

### Frontend (Jest / RTL — component tests where the pattern already exists)

- `RecommendButton` renders filled when `yourRecommendation === true`, outline otherwise.
- Optimistic update: count and toggle state update on tap before the mocked API resolves.
- Rollback on failure: count and toggle revert on 4xx/5xx and a toast surfaces.
- `RecommendedBy` renders nothing when the recommenders list is empty; renders first 3 avatars +
  "and N others" affordance when there are more.
- "and N others" opens a popover with all remaining names.

### E2E (`apps/bookshelf-e2e/tests/`)

One spec exercising the full loop against real servers: sign in, view catalog, tap heart on a
card, count increments and heart fills; sort by `Most Recommended` and confirm ordering; open
the book's detail page, confirm the recommender appears in the facepile; tap heart to remove;
confirm count decrements and the facepile drops the entry.

## Build order

1. Backend, in one PR:
   1. Migration + `Recommendation` model.
   2. Repository (`recommendation_repo.go`) — all methods listed above, tests first.
   3. Handler (`recommendations.go`) — three routes, tests first.
   4. `books.go` — extend `bookResponse` with `RecommendationCount` and `YourRecommendation`;
      populate both in `toBookResponse` and `toBooksResponse`; add `sort=recommended` in
      `buildListQuery`; tests.
   5. Cascade wiring — `deleteUser` in `admin.go` and `maybeDeleteOrphanedBook` in `copies.go`;
      tests.
   6. `cmd/server/main.go` — construct and register the recommendation handler; pass the repo
      into `NewBookHandler`.
2. Frontend, in a second PR (depends on backend PR being merged):
   1. Extract `InitialsAvatar`; switch `CopyCard.tsx` to it. No behavior change.
   2. `RecommendButton`, `RecommendedBy` components + component tests.
   3. `Book` type in `src/lib/types.ts` gains the two optional fields; `src/lib/api.ts` gains
      `recommendBook`, `unrecommendBook`, `getRecommendations`; add `recommended: "Most
Recommended"` to `SORT_LABELS`.
   4. `BookCard` integration — button + count, no facepile.
   5. `catalog/page.tsx` — new sort option.
   6. `catalog/[bookId]/page.tsx` — button + facepile below Copies, respecting the
      empty-coordination rule with Feature A's stats row.
   7. E2E spec.
3. Landing screenshots — if any of `apps/bookshelf`'s landing screenshots feature interactive
   book cards, seed one recommendation into `screenshot-seed.db` and regenerate via the tooling
   documented in `apps/bookshelf-e2e/CLAUDE.md`.

## Critical files

**Backend (new):**

- `internal/db/migrations/000015_create_recommendations.{up,down}.sql`
- `internal/repository/gorm/recommendation_repo.go` (+ test)
- `internal/handlers/recommendations.go` (+ test)

**Backend (modified):**

- `internal/models/models.go`
- `internal/repository/repository.go` (interface additions)
- `internal/handlers/books.go` (+ test) — response fields, sort case, batch calls
- `internal/handlers/admin.go` (+ test) — deleteUser cascade
- `internal/handlers/copies.go` (+ test) — maybeDeleteOrphanedBook cascade
- `cmd/server/main.go` — wire the handler and repo

**Frontend (new):**

- `src/components/InitialsAvatar.tsx`
- `src/components/RecommendButton.tsx`
- `src/components/RecommendedBy.tsx`

**Frontend (modified):**

- `src/components/CopyCard.tsx` — switch to `InitialsAvatar`
- `src/components/BookCard.tsx` — add `RecommendButton`
- `src/lib/types.ts`, `src/lib/api.ts`
- `src/app/catalog/page.tsx`, `src/app/catalog/[bookId]/page.tsx`

**E2E (new):**

- `apps/bookshelf-e2e/tests/book-recommendations.spec.ts` (or wherever the suite's convention
  lives)

## Verification

Before opening the backend PR:

- `pnpm nx test bookshelf-backend`
- `pnpm nx lint bookshelf-backend`

Before opening the frontend PR:

- `pnpm nx test bookshelf`
- `pnpm nx lint bookshelf`
- `pnpm nx e2e bookshelf-e2e`

Full gate for both, run before merge:

- `pnpm nx affected -t lint test e2e`

## What this plan doesn't cover

- **`nx release` changelog.** Automatic — the migration triggers the `### Database migrations`
  subsection via `tools/bookshelf-changelog/`. No manual `CHANGELOG.md` edit unless the default
  boilerplate is wrong for this migration (it isn't — the migration is trivial and idempotent).
- **Product-scope guardrail amendment.** Copy already drafted in the spec's "Scope-guardrail
  decision" section. Land that edit to both `apps/bookshelf/CLAUDE.md` and
  `apps/bookshelf-backend/CLAUDE.md` as part of the backend PR.
- **Notifications, tags, threshold badges.** All explicit v1 non-goals — see spec.
