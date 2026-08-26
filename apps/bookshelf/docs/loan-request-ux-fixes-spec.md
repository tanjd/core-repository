# Loan Request Flow — UX fixes spec

**Status:** Partially implemented — Fixes 1–4 shipped, Fix 5 outstanding · **Scope:**
`apps/bookshelf` + `apps/bookshelf-backend` · **Depends on:** `Copy`, `LoanRequest`,
`WaitlistEntry`, `Notification`

A UX pass over the borrow/loan/return flow (catalog → request → accept/reject → return, plus the
waitlist) surfaced five concrete gaps. Each is scoped independently below — pick and build in any
order, though **Fix 1** and **Fix 2** are the cheapest and highest-impact and should go first.

Fixes 1–4 shipped together in #71 ("loan request UX fixes"), including
`apps/bookshelf-e2e/src/loan-request-flow.spec.ts` coverage. Fix 5 (waitlist holds/queue) has not
been built — see its "Open decision" below on whether it's worth the scope.

## Fix 1 — "Requested" copies are a dead end for everyone but the requester ✅ Implemented (#71)

**Priority: high · Frontend + small backend change**

**Problem.** The instant a request is submitted, `CreateAndMarkRequested` flips the copy's status
straight from `available` to `requested` (a DB-transaction guard against double-booking) — not to
`loaned`. But every piece of "can I do something with this copy" logic only checks for
`available` (→ show "Request to Borrow") or `loaned` (→ show "Join Waitlist"):

- `WaitlistHandler.join` (`internal/handlers/waitlist.go`) 400s unless `bookCopy.Status ==
"loaned"`.
- `catalog/[bookId]/page.tsx`'s `canRequest`/`canWaitlist` mirror that same two-state check.

So while an owner sits on a pending request — which could be minutes or weeks — every other
interested member sees a "Requested" badge and **no button at all**. They can't join a waitlist,
can't register interest, nothing. If the request is later rejected or cancelled, nobody who was
watching finds out (see the notification gap below).

**Change.**

- Backend: relax `WaitlistHandler.join`'s guard from `Status == "loaned"` to `Status == "loaned"
|| Status == "requested"`. `WaitlistEntry` already just holds `(copy_id, user_id)` — no schema
  change needed.
- Frontend (`catalog/[bookId]/page.tsx`): change `canWaitlist`'s `isLoaned` check to
  `copy.status === "loaned" || copy.status === "requested"`.
- `CopyCard.tsx`: when `status === "requested"`, render a line of explanatory copy alongside the
  Join Waitlist button — e.g. "Someone's already asked to borrow this — join the waitlist to be
  notified if it opens up" — so the state reads as "temporarily spoken for," not a mystery dead
  end.

**Open decision — does the waitlist get notified when a `requested` copy reverts to
`available`?** `OnRejected`/`OnCancelled` (`internal/services/loan_workflow.go`) already flip the
copy back to `available` when the last pending request goes away, but neither calls
`notifyWaitlistAndClear` — that only fires from `OnReturned`. Recommend: extract the
notify-and-clear call so `OnRejected`/`OnCancelled` invoke it too (guarded on `pendingCount == 0`,
same condition already gating the status flip) — otherwise a member who joined the waitlist during
the "requested" limbo has no way to learn the copy opened back up short of manually re-checking.

## Fix 2 — Auto-approve is invisible, and the confirmation lies when it fires ✅ Implemented (#71)

**Priority: high · Frontend only**

**Problem.** A copy can have `auto_approve` set, in which case `finalizeLoanRequest`
(`internal/handlers/loan_requests.go`) skips the owner-review step entirely and returns the
request already `accepted`. Nothing in the UI signals this ahead of time — `CopyCard` doesn't
surface the flag, and the request dialog's copy ("You can include an optional message") reads
identically whether the copy is auto-approve or not. Worse, the frontend's success toast is
hardcoded to `"Borrow request sent!"` regardless of the response — a borrower on an auto-approve
copy has no idea their request was just instantly approved and the owner's contact info is now
available.

**Change.**

- `CopyCard.tsx`: when `copy.auto_approve` is true, show a small badge/label near the
  availability badge (e.g. "Instant approval") so it's visible before a member opens the request
  dialog.
- `catalog/[bookId]/page.tsx`'s request dialog: when `selectedCopy.auto_approve`, swap the
  description text to something like "This copy auto-approves — you'll get the owner's contact
  info right away."
- `handleRequest`: `api.createLoanRequest` already returns the full `LoanRequest` (its `status`
  field reflects the post-`finalizeLoanRequest` value — `"accepted"` for auto-approve, `"pending"`
  otherwise, per `createLoanRequest`'s handler in `internal/handlers/loan_requests.go`). Branch
  the toast on `response.status === "accepted"` → `"Request approved — check My Requests for the
owner's contact info"` vs. the current `"Borrow request sent!"` for the pending case. No backend
  change needed — the data is already there, just unused.

## Fix 3 — No visibility into overdue loans ✅ Implemented (#71)

**Priority: medium · Frontend only for v1**

**Problem.** `expected_return_date` is optional unless the owner sets `return_date_required`, and
even when present, nothing in the UI distinguishes "due in 3 days" from "was due 3 weeks ago."
`ReturnDateCell.tsx` renders the date (or "No return date agreed") with the same muted styling
regardless of how overdue it is.

**Change (v1, frontend only).**

- `ReturnDateCell.tsx`: when `request.status === "accepted"` and `expected_return_date` is in the
  past, render the date in `text-destructive` with a small "overdue" badge, matching the existing
  `success`/`destructive`/`secondary`/`outline` badge vocabulary (`apps/bookshelf/CLAUDE.md`).
  Apply the same styling everywhere this component is used: `my-requests/page.tsx`,
  `my-books/[copyId]/requests/page.tsx`, and `CurrentlyBorrowedCard.tsx`'s "Due …" line.
- `my-books/page.tsx`'s per-copy "Loaned to … · due …" line: same overdue treatment.

**Non-goal (v1) — reminder emails/notifications.** An automated "your loan is due soon /overdue"
nudge would need a scheduled sweep (following the existing `RegisterJob` pattern used by
cover-refresh and backups, per `apps/bookshelf-backend/CLAUDE.md`) plus new notification/email
copy. Worth doing once the visual overdue indicator ships and it's clear members still aren't
returning books on time — don't build the automation speculatively.

## Fix 4 — Owner can't counter-propose a return date ✅ Implemented (#71)

**Priority: medium · Frontend only**

**Problem.** A borrower proposes `expected_return_date` at request time (when
`return_date_required` is on); the owner's only options on a pending request are Accept or
Decline the whole thing. If the owner disagrees with the proposed date, the entire request has to
be rejected and the borrower asked to resubmit with a different date — pure friction, and there's
already an endpoint that makes this unnecessary.

**Change.** `PATCH /loan-requests/{id}/return-date` (`updateExpectedReturnDate`) already lets
either party set/change the agreed date any time while a loan is `accepted` — `ReturnDateCell`
uses it today. Reuse it at accept time instead of adding a new endpoint:

- `my-books/[copyId]/requests/page.tsx`: change the pending-request "Accept" action to open a
  small dialog (same `Dialog` pattern as the existing return-condition dialog) pre-filled with the
  borrower's proposed `expected_return_date` (editable `Input type="date"`, optional if the
  request didn't include one). On confirm: call `updateLoanRequest(id, {status: "accepted"})`,
  then — only if the owner changed the date — `updateExpectedReturnDate(id, newDate)`.
- No backend change required.

## Fix 5 — Waitlist is "notify everyone, first click wins," not a queue ⬜ Not built

**Priority: medium-high, but the largest change · Backend + frontend**

**Problem.** `notifyWaitlistAndClear` (`internal/services/loan_workflow.go`) fires a
`waitlist_available` notification to **every** waitlisted user simultaneously the moment a copy
becomes available, then clears the whole list. `WaitlistButton.tsx`'s copy — "Added to waitlist —
you'll be notified when it's available" — reads like a queue with a guaranteed turn; in practice
it's a race between everyone notified at once, decided by nothing more meaningful than reaction
time.

**Change — hold-based promotion, one person at a time.**

- Data model: add `HeldForUserID *uint` and `HoldExpiresAt *time.Time` to `Copy`
  (`internal/models/models.go`), migration `000010_add_copy_hold_fields` (`000009` is the current
  last — adjust the number if Fix 5 lands after a spec that also claims `000010`). A copy with a
  non-nil `HeldForUserID` is available _only_ to that user until `HoldExpiresAt`.
- `notifyWaitlistAndClear` → rename/rework into `promoteNextWaitlisted`: instead of notifying
  every entry, pop the single oldest `WaitlistEntry` (`ListByCopyID` already needs to return
  ordered-by-`CreatedAt`; confirm/add that ordering), set `HeldForUserID`/`HoldExpiresAt` (e.g.
  `time.Now().Add(48 * time.Hour)` — exact window is a product call, not an engineering one) on
  the `Copy`, delete just that one waitlist entry, and send that one user the
  `waitlist_available` notification. Everyone else stays queued.
- `getRequestableCopy` (`internal/handlers/loan_requests.go`): a copy with a live hold is only
  requestable by `HeldForUserID`; anyone else gets the existing "copy is not available" 400.
- New scheduled job (same `RegisterJob` pattern as cover-refresh/backups,
  `internal/services/scheduler.go`): sweep copies where `HoldExpiresAt` has passed and
  `HeldForUserID` is still set with no new `LoanRequest` created since the hold started — clear
  the hold and call `promoteNextWaitlisted` again for the next person in line. Mirrors the
  existing `cover_refresh_interval`/`backup_interval` admin-configurable-interval pattern; a new
  `waitlist_hold_hours` `AppSetting` follows the same shape.
- Frontend: `WaitlistButton.tsx` and `CopyCard.tsx` need to distinguish three states for a
  `loaned`/`requested` copy — "open to join," "you're being held a copy, N hours left to request
  it," and "someone else is holding it" — instead of today's binary on/off waitlist toggle.
  `getWaitlistStatus` gains a `held: boolean` / `hold_expires_at?: string` on the response.

**Open decision — is this worth the scope?** This is meaningfully bigger than Fixes 1–4 (new
columns, a new scheduled job, three new UI states) for a self-hosted app with a small community
catalog where waitlist contention is probably rare. Recommend scoping this one separately and
confirming there's an actual fairness complaint before building it — the other four fixes stand
on their own regardless.

## Build order

1. Fix 2 (auto-approve visibility) — pure frontend, no coordination with anything else.
2. Fix 1 (requested-state dead end) — small backend guard change + frontend, independent of Fix 2.
3. Fix 3 (overdue styling) — pure frontend, independent.
4. Fix 4 (counter-propose date at accept) — pure frontend, independent.
5. Fix 5 (waitlist holds) — build last, and only after confirming the open decision above; it's
   the one fix that isn't a same-day change.

## Verification

- Backend changes (Fixes 1 and 5): `pnpm nx test bookshelf-backend`, `pnpm nx lint
bookshelf-backend` (root `.golangci.yaml` enables `gocognit`/`gosec`/`revive` — keep
  `promoteNextWaitlisted` and the new sweep job small, same split pattern as `createLoanRequest`).
- Frontend: exercise manually via `pnpm nx dev bookshelf` against a local backend — for Fix 1 and
  5 specifically, this needs two logged-in members in different browser profiles/incognito windows
  to see the second party's view of a `requested`/held copy.
- End-to-end: extend `apps/bookshelf-e2e` for at minimum Fix 1 (second member sees a Join Waitlist
  option on a `requested` copy) and Fix 5 (hold expiry promotes the next waitlisted user) — both
  are the kind of two-actor, state-transition behavior that's easy to silently regress and hard to
  catch by manual spot-check.
- Full gate: `pnpm nx affected -t lint test` before merging any of the above.
