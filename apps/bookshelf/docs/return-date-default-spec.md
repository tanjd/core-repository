## Status: Draft — awaiting review · Scope: `apps/bookshelf` + `apps/bookshelf-backend` · Depends on: `Copy`, `LoanRequest` (existing — schema change to `Copy`, no new tables)

Every loan gets a real expected return date. The per-copy `return_date_required` opt-out goes away,
the borrower-side field is always shown with a sensible default of **30 days**, and the accept-side
counter-date flow becomes the single source of truth for the final agreed date. This closes the null
`expected_return_date` code path everywhere downstream (overdue tracking, reminders, admin views)
and gives community lending the soft accountability its social model depends on.

## Why now

The current model treats "no return date" as a legitimate first-class outcome — `Copy.return_date_required`
defaults to `false`, so most loans go through without any date attached. Two problems fall out of that:

1. **Silent failure mode.** A loan with `expected_return_date = NULL` cannot be flagged overdue by
   any query that compares against today. It vanishes from the accountability surfaces the product
   depends on (see `loan-request-flow.spec.ts`'s "overdue" test for the codepath that only fires
   when a date exists). In a community trust product, "no reminder ever fires" is the worst
   possible outcome — worse than a wrong reminder, because there's no forcing function to
   rediscover the loan.
2. **Configurability we can't defend.** The per-copy toggle looks like flexibility but is really
   the app declining to have a point of view. There's no evidence owners deliberately picked
   `false` on some copies and `true` on others — the default just propagated. Every downstream
   screen (borrower request dialog, owner accept dialog, overdue banners, reminder cron) pays a
   permanent tax to accommodate a choice users don't remember making.

Instrumentation from the last quarter (to be added — see "Rollout" below) is expected to show that
the `return_date_required=false` cohort has materially worse return rates than the `true` cohort.
If it doesn't, this spec should be reconsidered — but the null-handling code liability alone is a
sufficient reason to unify the path even if return rates are equal.

## Goals

- **Every accepted loan has a non-null `expected_return_date`** — enforced at the DB layer.
- **The borrower always sees the date field**, prefilled with `today + 30 days`, min `today`,
  editable within reason.
- **The `Copy.return_date_required` toggle is removed** from the schema, API, owner setup UI, and
  every conditional it currently gates.
- **Either party can change the expected return date after the loan starts**, without an
  approval workflow — a lightweight in-app edit that mirrors the side-channel conversation
  they've probably already had.
- **The owner's freeform expectations reach the borrower before they pick a date** — by echoing
  `copy.notes` inside the request dialog itself, not by adding a new structured field.
- **Existing rows are cleanly migrated** — no null dates left on any accepted/loaned loan after
  the migration runs.
- **Zero regressions** to the accept/counter-date flow, overdue tracking, or reminder cron.

## Non-goals (v1)

- **A "request extension" state machine.** Owner and borrower already talk via side channels
  (WhatsApp, Telegram, in person). What they need in-app is not a propose→approve workflow but a
  simple _field edit_ that either party can perform and that notifies the other — see the
  "Modifying the expected return date after a loan starts" section below. Naming this
  "extension" would drag in library/rental-app expectations that don't fit community lending.
- **A separate owner "nudge borrower" affordance.** Deferred — for v1 the notifications
  emitted when the expected return date changes (or when a loan goes overdue) are the nudge.
- **A per-copy owner-set expected return date** (e.g., `Copy.default_loan_days`). Explicitly
  ruled out. `Copy.notes` already exists as the freeform channel for expectations
  ("prefer 30 days — happy to extend"), and the owner can counter the borrower's date on accept
  for anything they want enforced. Adding a structured per-copy default rebuilds the
  configurability we're removing with `return_date_required`, introduces a naming collision with
  `LoanRequest.expected_return_date`, and creates a new field most owners won't remember setting
  — the same anti-pattern with a new column name. If instrumentation later shows owners
  routinely countering to identical dates across accepts on the same copy, we'll revisit; not
  before.
- **Per-owner or per-community default overrides.** No `default_loan_days` setting anywhere. If
  30 days turns out to be wrong for this community, we change the constant, not the surface area.
- **Calendar-aware defaults.** "One month" is defined as 30 days, not "the same day next month" —
  simpler, matches the existing `DEFAULT_LOAN_DAYS` constant pattern in the frontend, avoids
  month-boundary edge cases (Feb 30 → Mar 2, etc.).
- **A backend-configurable minimum/maximum loan length.** Deferred; the frontend only enforces
  `min=today` and the backend accepts any date `>= loan.accepted_at`. Community norms decide the
  soft ceiling; if abuse becomes real, we add a max later.

## The default: 30 days

Named `DEFAULT_LOAN_DAYS = 30` in both the frontend constants and backend config. Chosen because:

- 14 days is a library/hold-shelf convention that doesn't map to a community context — members
  aren't racing a queue, they're reading around life.
- 30 days matches the median self-reported book-reading window (per general reader-survey data,
  cite when this is real research not vibes) and aligns with monthly rhythms (book clubs,
  community groups, admin cadence).
- It's still short enough that a genuinely-forgotten book gets a reminder within a month rather
  than a quarter. If a copy sits unreturned for 30+ days _plus_ a reminder cycle, that's real
  signal, not noise.

If instrumentation post-launch shows a lot of borrowers manually shorten the date, we lower the
default. If borrowers routinely extend past 30 days, we raise it. **The default is tunable code,
not a database column** — this is the whole point.

## Modifying the expected return date after a loan starts

The most common real-world need — "we agreed I'd hold onto it a bit longer" — is a _field edit_,
not a workflow. This spec formalises it.

**Rules:**

- **Editable when**: `LoanRequest.status = 'accepted'` (i.e., the copy is out but not yet
  returned). Not editable while `pending` (still negotiating the initial accept — use the existing
  request/counter flow) and not on terminal statuses (`returned`, `rejected`, `cancelled`).
- **Who can edit**: either party — the borrower **or** the copy owner. No approval workflow. The
  reasoning: they've already talked via WhatsApp/Telegram/in-person. Adding an in-app
  propose→accept round trip duplicates a conversation they've already had; the notification below
  is the prompt to have that conversation if they haven't yet.
- **Guardrails**: new date must be `>= today` (can't retroactively make a loan overdue by
  backdating). No maximum for v1 — community norms self-police.
- **Notification**: the other party gets a `Notification` of a new type
  `expected_return_date_changed`, carrying the old date, the new date, and who changed it. Uses
  the existing bell/panel UI; no new surface.
- **Audit**: two new nullable columns on `LoanRequest`:
  - `expected_return_date_changed_by: uint` (FK to `users.id`, nullable — null means the original
    accept-time value has never been amended).
  - `expected_return_date_changed_at: DATETIME` (nullable, same rule).

  Last-write-wins. We do not keep full history — for a two-person negotiation that already lives
  in side-channel chat, more auditability isn't worth the schema and UI cost.

- **Rate limit**: none for v1. If we ever see 10+ changes on one loan in a week (visible via the
  audit fields), it's a signal to add friction, not to plan for.

**API:**

The route already exists — `PATCH /loan-requests/{id}/expected-return-date` — used today only by
the overdue e2e test as a setup helper (`loan-request-flow.spec.ts` L262–269). This spec promotes
it from "test setup helper" to "user-facing endpoint" without changing its request/response
shape:

| Route                                            | Change                                                                                                                                                                                     |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PATCH /loan-requests/{id}/expected-return-date` | Authorisation broadens: previously owner-only (implicit — no borrower UI called it). After: either the copy owner or the loan's borrower.                                                  |
|                                                  | Rejects request if `status != 'accepted'` (400).                                                                                                                                           |
|                                                  | Rejects request if new date `< today` (400).                                                                                                                                               |
|                                                  | On success: writes `expected_return_date_changed_by` + `expected_return_date_changed_at`, emits an `expected_return_date_changed` notification to the party who did _not_ make the change. |

## Data model changes

**Removed** (breaking):

| Field                         | Type   | Notes                                                                           |
| ----------------------------- | ------ | ------------------------------------------------------------------------------- |
| `copies.return_date_required` | `bool` | Dropped from schema. Every `Copy` after migration behaves as if this were true. |

**Added** (both nullable, both audit-only):

| Field                                           | Type        | Notes                                                            |
| ----------------------------------------------- | ----------- | ---------------------------------------------------------------- |
| `loan_requests.expected_return_date_changed_by` | `uint?`     | FK to `users.id`. Null = never amended after the initial accept. |
| `loan_requests.expected_return_date_changed_at` | `DATETIME?` | Null = never amended after the initial accept.                   |

**Tightened** (non-breaking to the type but breaking to callers relying on null):

| Field                                | Type   | Before   | After    | Notes                                                             |
| ------------------------------------ | ------ | -------- | -------- | ----------------------------------------------------------------- |
| `loan_requests.expected_return_date` | `DATE` | NULLable | NOT NULL | Enforced at DB level via migration `ALTER COLUMN … SET NOT NULL`. |

Frontend `Copy` type in `src/lib/types.ts`: drop the `return_date_required?: boolean` field.
Backend `Copy` struct: drop the `ReturnDateRequired` column and its migration entry.

Frontend `LoanRequest` type: add `expected_return_date_changed_by?: number` and
`expected_return_date_changed_at?: string` — used by the UI to show a small "Amended by
{name} on {date}" line beneath the current return date, so both parties can see the history at a
glance without a separate audit page.

Frontend `Notification.type` union: add `expected_return_date_changed`.

## Migration plan

Single Goose (or backend's equivalent) migration, run in one transaction:

1. **Backfill any null `expected_return_date`s on non-terminal loans** (`status IN ('pending',
'accepted')`):
   - For `status = 'pending'`: set to `requested_at + 30 days`. Borrower can amend on the next
     request only after cancelling; we notify these borrowers separately (see "Rollout"). This is
     tolerable because pending loans are rare and short-lived.
   - For `status = 'accepted'`: set to `COALESCE(loaned_at, responded_at, accepted_at, NOW()) + 30 days`
     — whichever of those is present, plus 30 days. The migration script logs the count.
2. **Backfill any null `expected_return_date`s on `returned` loans** to `returned_at` (they're
   already returned; the value is historical bookkeeping only, and setting it to the actual return
   date is the least-surprising choice).
3. **Backfill `rejected` and `cancelled` loans** to a sentinel value or leave with a temporary
   default (`requested_at + 30 days`) — they're terminal, but `NOT NULL` doesn't accept null. The
   value never surfaces to a user.
4. **Alter `expected_return_date` to `NOT NULL`.**
5. **Drop `copies.return_date_required` column.**

Migration is one-way — no `Down` beyond restoring the dropped column with `DEFAULT false`. The
`expected_return_date` values, once backfilled, stay. Team acknowledges this in the migration
review.

Migration must run **before** the frontend/backend code that assumes non-null dates ships, or
during the same deployment window with the app in maintenance mode. Given this app's low-write
volume (dozens of loans per week) a five-minute window is enough.

## API changes

| Route                                            | Before                                                                        | After                                                                                                                                                                                                                                                                                               |
| ------------------------------------------------ | ----------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /loan-requests`                            | `expected_return_date` optional; required only if `copy.return_date_required` | `expected_return_date` **required**, must be `>= today`. Missing → 400.                                                                                                                                                                                                                             |
| `PATCH /loan-requests/{id}` (owner accept)       | `expected_return_date` optional; falls back to borrower's date or null        | `expected_return_date` **required** when transitioning to `accepted`. Falls back to borrower's date if omitted; both being null is a 500-should-not-happen after migration.                                                                                                                         |
| `PATCH /loan-requests/{id}/expected-return-date` | Owner-only in practice; sets any date or null                                 | Either borrower or owner may call; date must be `>= today` and loan `status='accepted'`. Null → 400. Writes `expected_return_date_changed_{by,at}` and emits an `expected_return_date_changed` notification to the other party. See "Modifying the expected return date after a loan starts" above. |
| `POST /copies`                                   | Accepts `return_date_required`                                                | Ignores the field (silently, for one release cycle) then drops it from the schema entirely.                                                                                                                                                                                                         |
| `GET /copies/*`, `GET /books/*`                  | Copies include `return_date_required`                                         | Field removed from all `Copy` responses.                                                                                                                                                                                                                                                            |

Backend validation errors are surfaced as toast messages on the frontend using the existing
error-envelope pattern (`{"error": "..."}`) — no new error shape.

## Frontend changes

### `apps/bookshelf/src/app/catalog/[bookId]/page.tsx`

- **Remove** the `{selectedCopy?.return_date_required && (...)}` conditional. Always render the
  return-date field in the request dialog.
- **Default value**: `today + 30 days`, computed via a shared `DEFAULT_LOAN_DAYS = 30` constant.
- **Min**: today. **Max**: none (let community norms self-police for v1).
- **Helper text**: _"You can propose a different date. The owner can counter this when they
  accept."_ Replaces the current `return_date_required`-only helper.
- **Echo `selectedCopy.notes` inside the dialog** (when present), styled as an owner's quote
  above the message textarea — e.g., a bordered `<blockquote>` with the notes in italics and a
  "— shared by {owner.name}" attribution when not hidden. This closes the "borrower doesn't see
  the owner's expectations at request time" gap without a new field. Empty notes → block hidden.
- **Request submission**: always send `expected_return_date` — the ternary that made this
  conditional on `return_date_required` goes away.
- **CTA label**: no change — `Borrow instantly` / `Request to Borrow` still apply.

### `apps/bookshelf/src/app/my-requests/page.tsx` (borrower's list)

- On each **accepted** loan row, add a "Change return date" affordance — same shadcn `Dialog`
  shape as the borrow request, prefilled with the current `expected_return_date`, min today.
- On submit: `PATCH /loan-requests/{id}/expected-return-date`, toast "Return date updated —
  {owner.name} has been notified", refresh the row.
- Beneath the return date, if `expected_return_date_changed_at` is set, show a small muted line:
  _"Amended {relative time} ago"_ so the borrower can see it changed without opening the dialog.

### `apps/bookshelf/src/app/my-books/[copyId]/requests/page.tsx` (owner's list)

- Mirror of the above. On each accepted loan, "Change return date" opens the same dialog shape,
  prefilled with the current value.
- `ReturnDateCell.tsx` already renders the current date on this page — extend it to show the
  same "Amended by {borrower.name}" muted line when the audit fields are set.
- The **existing accept dialog** (`#accept-return-date`) is unchanged in visibility — always
  shown, prefilled from the borrower's date. This is v1's primary "set the initial value" path;
  the new post-accept edit path is for genuine changes after the loan starts, not for adjusting
  the initial value.

### `apps/bookshelf/src/app/my-books/[copyId]/setup.tsx` (or wherever copy creation lives)

- Remove the `Return date required` toggle from the copy-setup form.
- Remove any helper text that mentioned the toggle.
- Existing owners see one fewer question in the setup flow.

### `apps/bookshelf/src/app/my-books/[copyId]/requests/page.tsx` (owner accept dialog)

- The accept dialog's `#accept-return-date` field is **already always shown** — no change to
  visibility, but its default now comes from the borrower's submitted date (guaranteed non-null
  after migration) rather than an occasionally-null value.
- Min: today. Owner can counter to any date `>= today`.

### `apps/bookshelf/src/lib/types.ts`

- `Copy.return_date_required` field removed.
- `LoanRequest.expected_return_date` changed from `string | undefined` to `string` (non-optional).
- `LoanRequest` gains `expected_return_date_changed_by?: number` and
  `expected_return_date_changed_at?: string`.
- `Notification.type` union gains `expected_return_date_changed`.

### `apps/bookshelf/src/components/NotificationPanel.tsx`

Render the new `expected_return_date_changed` notification type — one new `case` in the existing
`switch` that formats notification rows. Text: _"{name} changed the return date on {book title}
to {new date}."_ Deep-links to the loan (borrower → `/my-requests`, owner →
`/my-books/[copyId]/requests`) depending on the current user's role, same routing pattern as
`request_accepted` / `marked_loaned` / etc.

### Nowhere else: no new pages, no new components — the two new dialogs reuse the borrow-request

dialog shape and shadcn primitives.

## Instrumentation

Add before shipping the removal, keep for 4 weeks after:

- **Borrower's submitted `expected_return_date - today` in days** on every `POST /loan-requests`.
  Aggregate: "how often does the borrower change the default?" If >60% accept `30` unchanged, the
  default is well-calibrated. If <20% accept it, the default is wrong.
- **Owner-counter delta on accept**: `owner_expected_return_date - borrower_expected_return_date`
  in days. Answers: "do owners systematically extend or shorten the borrower's proposal?"
- **Post-accept edit rate + delta**: count of loans where
  `expected_return_date_changed_at IS NOT NULL`, and the median +/- delta of the change. This is
  the "how often do people actually use the edit affordance, and does it tend to extend or
  shorten?" signal. If the affordance is rarely used, we've overbuilt it; if it's heavily used to
  extend, 30 days may be too short.
- **Return-rate metric**: `% of loans returned within (expected_return_date + 7 days)`, tracked
  weekly. This is the outcome metric — the whole spec exists to move this number.

All four go to the existing admin dashboard (`apps/bookshelf/src/app/admin/`), not a new tool.

## Build order

1. **Backend migration** (`internal/repository/gorm/migrations/`):
   - Backfill nulls per the "Migration plan" section.
   - `ALTER COLUMN loan_requests.expected_return_date SET NOT NULL`.
   - `ADD COLUMN loan_requests.expected_return_date_changed_by BIGINT NULL REFERENCES users(id)`.
   - `ADD COLUMN loan_requests.expected_return_date_changed_at TIMESTAMP NULL`.
   - `DROP COLUMN copies.return_date_required`.

   Migration test alongside covers the backfill formula per status and the additive columns.

2. **Backend model + repo**: drop `Copy.ReturnDateRequired` field. Tighten
   `LoanRequest.ExpectedReturnDate` to non-optional. Add `ExpectedReturnDateChangedBy` and
   `ExpectedReturnDateChangedAt` fields. Repo tests updated.
3. **Backend handlers**:
   - `POST /loan-requests`: enforce required, `>= today`. 400 otherwise.
   - `PATCH /loan-requests/{id}` (accept): unchanged in shape; ensure the ternary
     "borrower-date-or-null" collapses to just "borrower-date" now.
   - `PATCH /loan-requests/{id}/expected-return-date`: broaden authorisation to
     `owner OR borrower`; enforce `status='accepted'` and `date >= today`; write audit columns;
     emit notification of type `expected_return_date_changed` to the other party.
   - `POST /copies`: strip `return_date_required` from request/response shapes.

   Handler tests cover every 400 path plus the new notification emission and the audit
   column writes.

4. **Frontend types + API** (`src/lib/types.ts`, `src/lib/api.ts`): type changes as above; add
   `api.updateExpectedReturnDate(id, date)` if not already exposed (only used by e2e today).
5. **Frontend UI**:
   - `catalog/[bookId]/page.tsx`: always-show date field, echo `copy.notes` inside the dialog,
     update helper text.
   - Copy-setup form: remove the toggle.
   - `my-requests/page.tsx` and `my-books/[copyId]/requests/page.tsx`: add the "Change return
     date" dialog + amended-by muted line.
   - `NotificationPanel.tsx`: render the new notification type.
6. **Instrumentation**: log lines + admin dashboard tiles (four metrics per the section above).
7. **E2E** (`apps/bookshelf-e2e/src/loan-request-flow.spec.ts`):
   - Existing "counter date" test already covers accept-time counter — keep as-is.
   - Add: borrower request with no explicit date defaults to +30d.
   - Add: copy-setup form has no return-date toggle.
   - Add: **borrower changes the return date post-accept**, owner sees the notification and the
     amended-by line.
   - Add: **owner changes the return date post-accept**, borrower sees the notification and the
     amended-by line.
   - Add: attempting to change the date on a `returned`/`rejected`/`cancelled` loan returns 400.
   - Remove: the auto-approve test's assumption that no-date-required copies exist.

## Rollout

- **Comms**: one-time in-app announcement (using the existing `Announcement` machinery — see
  `apps/bookshelf/src/lib/announcements.ts`) explaining that every borrow now has a soft return
  date defaulting to 30 days. Text drafted by product; targets both borrowers and owners.
- **Owner-facing note in copy-setup form** during the first release cycle after the toggle is
  removed: _"Return dates are now on for every borrow, defaulting to 30 days. You can still
  counter the date when you accept."_ Removed one release later.
- **Pending-loan backfill notification**: any borrower whose pending loan got a backfilled date
  gets a notification (existing `Notification` type, add a new `type` variant
  `expected_return_date_backfilled`). Owner sees the same on their side.

## Critical files

- Backend: `internal/repository/gorm/migrations/*`, `internal/models/copy.go`,
  `internal/models/loan_request.go`, `internal/repository/gorm/copy_repo.go`,
  `internal/repository/gorm/loan_request_repo.go`,
  `internal/handlers/copies.go`, `internal/handlers/loan_requests.go`, plus their `_test.go`
  companions.
- Frontend: `src/lib/types.ts`, `src/app/catalog/[bookId]/page.tsx`, copy-setup form (path TBD —
  needs confirmation before build), owner accept dialog.
- E2E: `apps/bookshelf-e2e/src/loan-request-flow.spec.ts`.
- Migration test coverage in the backend test suite.

## Verification

- Backend: `pnpm nx test bookshelf-backend`, `pnpm nx lint bookshelf-backend`, plus a manual
  migration dry-run against a snapshot of production data (or the closest available seed).
- Frontend: `pnpm nx test bookshelf`, `pnpm nx lint bookshelf`.
- E2E: `pnpm nx e2e bookshelf-e2e`.
- Full gate: `pnpm nx affected -t lint test e2e` before merging.
- Post-deploy: verify migration ran cleanly by querying `COUNT(*) FROM loan_requests WHERE
expected_return_date IS NULL` — should be 0.

## How we'll know it's working

- **Zero rows** where `expected_return_date IS NULL` after migration, forever.
- **Overdue banner** actually fires on stale loans (previously silent for null-date loans).
- **Return rate** (loans returned within expected date + 7 days) rises in the 4-week window
  post-launch — the outcome metric. If it doesn't, we haven't fixed the underlying accountability
  gap and the "how we communicate expectations" surface (notes echo, post-accept edits) needs
  another pass.
- **Default acceptance rate**: >50% of borrowers accept the 30-day default. If it's much lower,
  the default is miscalibrated; adjust the constant.
- **Post-accept edit usage**: the affordance is used at least occasionally (>5% of accepted
  loans) — proves it isn't dead code. If it's never used, that's evidence the side-channel
  conversation is enough and we can retire it in a follow-up.
- **No user complaints** about "I couldn't borrow without picking a date" — if we get more than
  a handful, the field's default-and-required framing needs revisiting (e.g., a one-tap
  "use 30 days" chip alongside the picker).

## Open questions for the reviewer

1. **Precise copy-setup form path.** Confirm the file/component where owners currently see the
   `return_date_required` toggle so this spec can name it precisely in the build steps.
2. **Backfill notification wording.** Do we want to email/notify borrowers with pending loans
   about the backfilled date, or is a silent 30-day default acceptable given how rarely a loan
   sits in `pending` for more than a few days?
3. **Announcement timing.** In-app announcement one week before the change, at the change, or
   both? Recommend both — pre-announce to owners (who need to know their setup form is changing),
   at-announce to borrowers (who see the new required field).
4. **Frontend fallback for legacy `return_date_required` in cached responses.** Any client cache
   or service-worker layer that would keep the old field alive briefly? If yes, plan a cache
   bust; if no, ignore.
5. **Notification cadence for post-accept date changes.** The spec sends one notification per
   change with no throttling. If two parties both edit the date within seconds (which shouldn't
   happen in practice but is theoretically possible), each gets a notification for the other's
   change. Acceptable, or worth a 5-minute debounce? Recommend acceptable — the audit trail is
   the source of truth; notifications are just prompts.
6. **Should the post-accept edit affordance also be available while the loan is `pending`?**
   Currently gated to `accepted` only. Argument for opening it up: a borrower might realise
   mid-negotiation that their proposed date was wrong. Argument against: `pending` already has
   its own negotiation shape (owner counters on accept), and adding a second edit path muddles
   ownership of the pre-accept date. Recommend keeping `accepted`-only for v1; revisit if
   borrowers report needing it.
