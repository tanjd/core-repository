# Loans (Borrowing + Lending) — spec

**Status:** Implemented · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` · **Depends on:**
`LoanRequest`, `Copy` (both existing — no new tables, no migration)

Give every member one place to see all their loan activity — what they've borrowed and what
they've lent — instead of the borrower half living on its own tab and the lender half not existing
at all.

## Why now

The borrower side of the exchange already has a dedicated page: `/my-requests`, with Current/
History tabs, pagination, and a due-date/overdue view (`getMyLoanRequests`, backed by
`ListByBorrowerIDPaginated`). The owner side has no equivalent — `GET /loan-requests?copy_id=X`
(`ListByCopyID`) only returns one copy's requests at a time, reached by opening
`/my-books/{copyId}/requests` per book. An owner with more than a couple of copies out has no way
to see "everyone who's borrowed from me" without clicking into each copy individually.

An earlier draft of this spec (see git history / superseded `lending-history-spec.md`) proposed
fixing just that second gap: a read-only "Lending History" page linked from `/my-books`, with
`/my-requests` left untouched. Working through the navigation surfaced a sharper problem —
**"My Requests" is already ambiguous.** It sounds like it could mean "requests on my stuff"
(incoming) just as easily as "requests I made" (outgoing), and it's the _only_ one of those two
that exists as a tab; the other lives buried under a different section. Borrowing and lending are
mirror images of the same transaction — giving one a first-class tab and the other a link on an
unrelated page treats two symmetric concepts asymmetrically, which is confusing regardless of how
either page is labeled internally.

This spec instead **replaces the "My Requests" tab with a "Loans" tab**, containing both
directions as sibling sub-tabs (**Borrowing** / **Lending**). One predictable destination, named
for what it contains rather than for one half of it.

## Goals

- A single nav destination (`/loans`, replacing `/my-requests`) covers all loan activity for the
  logged-in member, split into two peer sub-tabs:
  - **Borrowing** — everything the member has requested to borrow from others. This _is_ today's
    `/my-requests` page, moved and relabeled, behavior unchanged (Current/History sub-filter,
    `CurrentlyBorrowedCard`s, pagination, cancel/return actions).
  - **Lending** — everything requested to borrow copies the member owns, across their whole
    collection. New. Same Current/History sub-filter and pagination pattern as Borrowing, read-only
    (see Non-goals).
- The tab bar / nav item reads "Loans," not "Requests" or "My Requests" — removes the ambiguity
  entirely rather than just relabeling one half of it.
- Reuses the existing per-request data and redaction rules (`safeUser`, contact info revealed only
  once accepted) — no new PII exposure.
- Zero new tables, zero new columns.

## Non-goals (v1)

- **Deep-linking a specific sub-tab via URL.** `/loans` opens on Borrowing by default (matching
  where `/my-requests` links already point, e.g. from notifications — see Migration below); which
  role/Current-or-History sub-tab is open is plain component state, same convention the existing
  page already uses. Add query-param deep-linking later if it turns out people want to
  share/bookmark a specific view.
- **Manage-from-Lending actions (accept/reject/mark-returned).** Those already live on
  `/my-books/{copyId}/requests` and stay there — the Lending sub-tab is a read-only aggregate view
  across all copies, not a second place to manage an individual loan. Keeps the single-write-site
  discipline the backend already follows elsewhere in this app (see the wishlist workflow's
  "single write site" note in `apps/bookshelf-backend/CLAUDE.md`) — one page owns state
  transitions, this one just reads them.
- **An owner-side "active loans" glance view (a `CurrentlyBorrowedCard` equivalent) inside
  Lending.** `/my-books` already shows each copy's current status (`loaned`/`requested` badge +
  "Loaned to … · due …" line), so the Lending sub-tab's Current filter — a table row per active
  loan — covers the same data for anyone who wants it listed instead of per-card, without
  duplicating a glance surface.
- **Per-borrower aggregation** ("Jane has borrowed 4 books from you total"). A second-order view on
  top of the flat list — defer until the flat list ships and it's clear members want the rollup.
- **Export/CSV.** `/my-books` already has a copies export; a loan-history export is a plausible
  follow-up but no one's asked for it yet.

## API surface

One new route, modeled directly on the existing `list-my-loan-requests`:

| Route                         | Method | Notes                                                                                                                                                                                                                           |
| ----------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/loan-requests/mine/lending` | GET    | Owner-side mirror of `/loan-requests/mine`. Query params: `view` (`current`\|`history`, same `statusesForView` mapping), `page`, `page_size`. Auth required; scoped to `callerID` as the _owner_ of the copy, not the borrower. |

Response body is the existing `listMineOutput` shape (`items`/`total`/`page`/`page_size`/
`total_pages` of `getLoanRequestBody`) — no new DTO needed, since `getLoanRequestBody` already
includes both `Copy` (with `Book`) and `Borrower`, which is exactly what a Lending row needs to
render.

`/loan-requests/mine` (the existing borrower endpoint) is unchanged — it becomes the Borrowing
sub-tab's data source, same as it is today for `/my-requests`.

No changes to `GET /loan-requests?copy_id=X` — it stays as the per-copy, manage-this-queue view
used by `/my-books/{copyId}/requests`.

## Backend

- **Repository** (`internal/repository/repository.go` → `LoanRequestRepository` interface,
  `internal/repository/gorm/loan_request_repo.go`): add
  `ListByOwnerIDPaginated(ownerID uint, statuses []string, page, pageSize int) (*repository.PaginatedResult[models.LoanRequest], error)`,
  modeled on `ListByBorrowerIDPaginated` (`internal/repository/gorm/loan_request_repo.go:107`)
  with one structural difference: `owner_id` lives on `copies`, not `loan_requests`, so both the
  count and select queries need a join:

  ```go
  base := r.db.Joins("JOIN copies ON copies.id = loan_requests.copy_id").
      Where("copies.owner_id = ?", ownerID)
  ```

  **Gotcha:** both `copies` and `loan_requests` have a `status` column. Once the join is in place,
  an unqualified `Where("status IN ?", statuses)` is ambiguous and SQLite will reject it — qualify
  every status filter as `loan_requests.status IN ?` (the count query needs
  `.Model(&models.LoanRequest{})` _and_ the join, same as the select query, not just a bare
  `Where`). `Order("requested_at DESC")` needs no qualification since only `loan_requests` has that
  column.

  No new indexes needed — `idx_loan_requests_copy_id` and `idx_copies_owner_id`
  (`internal/db/migrations/000001_init.up.sql`) already cover the join and filter.

- **Handler** (`internal/handlers/loan_requests.go`): add `listMineLending`, a near-duplicate of
  `listMine` (`~line 285`) calling `ListByOwnerIDPaginated` instead of
  `ListByBorrowerIDPaginated`, reusing `statusesForView` and `toGetLoanRequestBody` unchanged.
  Register it in `RegisterRoutes` (`~line 148`) as `GET /loan-requests/mine/lending`, right after
  `list-my-loan-requests`.

## Frontend

### Page restructure

Implemented as **three components**, not one file with nested `Tabs`, as originally sketched
below — nearly every piece of state and every handler in `my-requests/page.tsx` (`requests`,
`page`, `tab`, `expanded`, `returnDialog`, `loadRequests`, `toggleExpand`, `handleCancel`, ...)
would collide by name if both tabs' content lived in one component scope, and this codebase has no
precedent for a shared list/table component (`my-requests/page.tsx` and
`my-books/[copyId]/requests/page.tsx` were already two independent, duplicated implementations of
the same Table+expand-row pattern before this change):

```
src/app/loans/
├─ page.tsx          ← shell: heading + outer Tabs(Borrowing/Lending), auth-redirect check
├─ BorrowingTab.tsx   ← moved from my-requests/page.tsx, own local state, unchanged behavior
└─ LendingTab.tsx     ← new, own local state, read-only
```

- **Move** `src/app/my-requests/page.tsx` → `src/app/loans/BorrowingTab.tsx`, exported as a named
  `BorrowingTab` component. The page-level auth-redirect `useEffect` and the outer heading/wrapper
  moved up to `page.tsx` instead of staying in `BorrowingTab` — a page-level concern, not
  per-tab.
- **Borrowing tab body**: today's `/my-requests` page content verbatim — `CurrentlyBorrowedCard`s,
  Table + expand-row, `Pagination`, cancel/return dialog actions, `statusVariant`/
  `conditionVariant` badge maps. No behavior change, just relocated into its own component.
- **Lending tab body**: new, adapted from the same Table + expand-row + Current/History +
  `Pagination` pattern, sourced from `api.getMyLendingHistory` (below) instead of
  `api.getMyLoanRequests`. Column swap: show **borrower** name/contact (`ContactReveal`, already
  redaction-aware) and **book** (cover + title, from `request.copy.book`) instead of owner. No
  action buttons (see Non-goals) — read-only rows. **`ReturnDateCell` is not reused as-is** — it
  has an inline edit affordance (`Popover` + `api.updateExpectedReturnDate`) that would make this
  tab writable, so `LendingTab.tsx` has its own small `returnDateDisplay()` helper that renders
  just the date + overdue badge, no edit control.
- **`src/lib/api.ts`**: add `getMyLendingHistory`, a copy-paste of `getMyLoanRequests` pointed at
  `/loan-requests/mine/lending` instead of `/loan-requests/mine`. Same
  `PaginatedResult<LoanRequest>` return type — no new types.

### Nav

- **`src/components/layout/navItems.ts`**: change the `primaryNavItems` entry —
  `href: "/my-requests"` → `"/loans"`, `label: "My Requests"` → `"Loans"`, `shortLabel: "Requests"`
  → `"Loans"` (short enough to not need shortening — drop the field), icon `ListChecks` → lucide's
  `Handshake` (reads as "an exchange between two people," and avoids visual overlap with the
  existing `ArrowRightLeft` "Transfer" icon already used elsewhere on `/my-books`). `isActive`
  becomes `(p) => p === "/loans"`.
- Drop the "Lending History" entry-point button that the earlier draft of this spec put on
  `/my-books`'s header — Lending is reachable from the Loans tab now, not from My Books.

### Notification routing

- **`src/lib/notifications.ts`**: every `"/my-requests"` return value (the `marked_returned`/
  `expected_return_date_changed` borrower branch at `~line 78`, and the catch-all default at
  `~line 87`) becomes `"/loans"`. Both land on the Borrowing sub-tab by default, which is correct —
  every notification type that reaches those branches is inherently a borrower-facing event.
  `request_received` and the owner branch of `marked_returned`/`expected_return_date_changed`
  already route to `/my-books/{copyId}/requests` and are unaffected — owner notifications still go
  to the manage-this-loan page, not to the read-only Lending aggregate.

## Migration notes

- `/my-requests` had no redirect added — no external backlinks to the old path in a self-hosted
  app, and `src/app/my-requests/` was deleted once `/loans` was verified working.
- `apps/bookshelf-e2e/src/loan-request-flow.spec.ts` had five `page.goto("/my-requests")` calls
  (not six, as an earlier pass of this plan estimated) updated to `/loans` — each already exercises
  Borrowing-tab behavior since Borrowing is the default tab. It also had a toast-copy assertion
  ("Request approved — check My Requests...") that had to change together with the same string in
  `src/app/catalog/[bookId]/page.tsx`, not independently.
- `apps/bookshelf-e2e/src/primary-navigation.spec.ts` had two separate assertions, not textually
  identical: the desktop link name ("My Requests" → "Loans") and the mobile tab bar's link name,
  which was asserting `shortLabel`'s value ("Requests"), not `label`'s — once `shortLabel` was
  dropped from the nav item, mobile renders the full "Loans" label too, so that assertion changed
  for a different reason than the desktop one, not as a mechanical find-replace of the same string.
- `apps/bookshelf/CLAUDE.md`'s "Mobile-first UI" section named the four tab-bar destinations in
  prose ("Catalog, My Books, My Requests, Wishlist") — updated to match.

## Build order

1. Repository: `ListByOwnerIDPaginated` + a repo test covering the join/ambiguous-column gotcha
   above (own copies only, status filter, pagination, empty result for a user who owns no copies).
2. Handler: `listMineLending` + route registration + handler test mirroring `listMine`'s.
3. Frontend data: `getMyLendingHistory` in `api.ts`.
4. Frontend restructure: move `my-requests` → `loans`, wrap in the Borrowing/Lending outer `Tabs`,
   build the new Lending tab body.
5. Nav + notifications: `navItems.ts` rename, `notifications.ts` route updates.
6. End-to-end: update `loan-request-flow.spec.ts`'s six `/my-requests` references to `/loans`;
   add a new case — seed two members, one lends a copy to the other and the loan is returned;
   verify the owner's Loans → Lending → History tab shows it and Current doesn't (and vice versa
   for a still-`accepted` loan).

## Critical files

- `internal/repository/repository.go`, `internal/repository/gorm/loan_request_repo.go`,
  `internal/repository/gorm/loan_request_repo_test.go`
- `internal/repotest/repotest.go` — in-memory fake repo every handler test builds against; needed
  its own `ListByOwnerID`/`ListByOwnerIDPaginated` implementation to keep the package compiling
- `internal/handlers/loan_requests.go`, `internal/handlers/loan_requests_test.go`
- `src/lib/api.ts`, `src/lib/notifications.ts`
- `src/app/loans/{page,BorrowingTab,LendingTab}.tsx` (`BorrowingTab.tsx` moved from
  `src/app/my-requests/page.tsx`, which was deleted)
- `src/components/layout/navItems.ts`
- `src/app/catalog/[bookId]/page.tsx` (toast copy)
- `apps/bookshelf/CLAUDE.md`
- `apps/bookshelf-e2e/src/loan-request-flow.spec.ts`,
  `apps/bookshelf-e2e/src/primary-navigation.spec.ts`

## Verification

- Backend: `pnpm nx test bookshelf-backend`, `pnpm nx lint bookshelf-backend` (root
  `.golangci.yaml`'s `gocognit` — `listMineLending`/`ListByOwnerIDPaginated` should come in well
  under threshold since they're near-duplicates of already-small functions).
- Frontend: `pnpm nx dev bookshelf` against a local backend, log in as a member who both has
  pending/accepted/returned/rejected requests as a borrower _and_ owns copies with a mix of
  request statuses, confirm both sub-tabs, their Current/History filters, and pagination.
- End-to-end: `pnpm nx e2e bookshelf-e2e`.
- Full gate: `pnpm nx affected -t lint test` before merging.

## How we'll know it's working

- Notification-driven navigation to `/loans` continues to work without a spike in support
  questions ("where did My Requests go?").
- The Lending sub-tab gets used (server logs on `/loan-requests/mine/lending`, per
  `middleware/logging.go`).
- Questions along the lines of "who did I lend book X to again?" stop coming up.
