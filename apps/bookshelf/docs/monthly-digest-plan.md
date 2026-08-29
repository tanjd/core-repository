# Monthly digest email — implementation plan

Companion to `monthly-digest-spec.md`. The spec is the behavior contract ("what and why").
This plan is the wiring against this specific codebase ("how"). If requirements shift, the spec
changes; if the codebase shifts, this plan changes.

**Prerequisite:** the spec's `Status:` header is Approved for build (it is).

## Design decisions the spec deliberately left open

### 1. Unsubscribe-token lifetime and purpose string

Spec says "long-lived (e.g. ~1 year) signed token, separate from the existing short-lived OTP-link
tokens." Concretely:

- New purpose constant `unsubscribe_digest` in `internal/handlers/otp_link_token.go`, alongside
  the existing `register_email_otp` / `reset_password` / `email_change` / `otp_verify`.
- The existing `issueOTPLinkToken` / `verifyOTPLinkToken` helpers hardcode `otpLinkTokenTTL =
15 * time.Minute` and always resolve an OTP code. Neither fits: the digest link has no code,
  and needs a ~1y TTL. Two options:
  1. Add a TTL parameter and a nil-code path to the existing helpers.
  2. Add a parallel pair (`issueUnsubscribeToken` / `verifyUnsubscribeToken`) that share the same
     signer/secret but never touch the OTP-code fields.
     **Going with option 2.** The existing helpers are shaped around "code + short window" and
     overloading them dilutes both surfaces; the shared HMAC signer is the only real reuse
     needed. Lifetime: `unsubscribeTokenTTL = 365 * 24 * time.Hour`. Payload carries only
     `user_id` + `purpose` + `iat`/`exp` — no email, no code (the spec explicitly notes "carries
     only the member's identity").
- Token is minted **at send time** per recipient, not stored. Revocation is implicit: flipping
  `MonthlyDigestEnabled=false` makes the next click a no-op ("already unsubscribed"), and rotating
  the signing secret invalidates every outstanding link.

### 2. Send loop: sequential, no budget tracking, no worker pool

Spec (updated): at a community ceiling of ~100 members the 300/day relay cap isn't a live
constraint, so v1 skips all the machinery that used to defend it. Concretely:

- `EmailService` is untouched — no new `TrySend`, no counter, no `AppSetting` keys for budget.
  Existing `SendEmail` stays the single choke point.
- The digest send loop is a plain `for _, u := range recipients { … }` calling `SendEmail`
  once per member, in-process, sequentially. `sent` and `failed` counters are incremented
  based on the returned error; both are surfaced in the job's result string on the admin
  Jobs page ("sent 87, failed 0").
- A per-send failure logs `user_id` + error + goes on to the next recipient — one member's
  SMTP-side rejection doesn't cost every subsequent member their digest. The month is still
  marked handled at the end regardless of `failed > 0` (spec's non-goal on delivery ledger /
  retry-safe resumability applies).
- If the relay ever starts rate-limiting, the individual `SendEmail` errors bubble up and
  the admin sees "failed N" on Jobs; that's the whole failure mode. Revisit if that becomes
  a routine occurrence.

### 3. New-books date-range and top-recommended queries: two new repo methods, not a filter on `ListPaginated`

`BookRepository.ListPaginated` in `internal/repository/gorm/book_repo.go` already handles
`sort="recommended"` via a subquery join on recommendation counts, but its filter surface (search
term, ownership) isn't shaped for arbitrary date ranges, and the digest doesn't need pagination
or the ownership metadata `ListPaginated` computes. Cleaner as two dedicated methods:

- `BookRepository.ListCreatedBetween(from, to time.Time, limit int) ([]Book, error)` — newest
  first (`created_at DESC`), preload `Copies` so the email template can show cover URL / title
  / author without an N+1.
- `RecommendationRepository.ListTopBooks(limit int) ([]TopRecommendedBook, error)` where
  `TopRecommendedBook{Book, Count}`. One `SELECT ... FROM books LEFT JOIN (SELECT book_id,
COUNT(*) FROM recommendations GROUP BY book_id) r ...` ordered `count DESC, title ASC` (matches
  the spec's tie-break rule and the existing "Most Recommended" sort in `ListPaginated`).
  Books with zero recommendations are excluded (spec's "top recommended", not "top including
  zeros").

### 4. Eligible-recipients query: new method on `UserRepository`, not a filter on `admin_repo.ListUsersPaginated`

`admin_repo.ListUsersPaginated` is unfiltered and paginated for the admin table. The digest wants
"stream every eligible member without pagination overhead":

- `UserRepository.ListDigestRecipients(ctx) ([]User, error)` — one query filtering
  `is_verified = 1 AND is_suspended = 0 AND is_pending_approval = 0 AND
monthly_digest_enabled = 1`. Selects only the columns the digest email needs (id, email, name)
  to keep the row size small.
- Lives on `UserRepository` (`user_repo.go`) rather than `AdminRepository` because it's not an
  admin surface — the digest service is the only caller.

### 5. Idempotency ledger: single `AppSetting` key, not a new table

Spec: "checks whether the current month has already been handled … marks the current month
handled." Cheapest: one `AppSetting` key `monthly_digest_last_handled_month` storing `YYYY-MM`
(the month the _previous-full-calendar-month_ digest was sent for, i.e. the month whose ticks
should stop). Comparison is a string equality against `time.Now().Format("2006-01")` — plain
`time.Now()`, which uses the process's local timezone (see decision 7 below). No new table;
matches how `backup_last_run_at` and cover-refresh state are already tracked via `AppSetting`.

### 6. Send-day setting semantics

Spec: "once the configured send day of the month has arrived." Concretely:

- New `AppSetting` key `monthly_digest_send_day`, default `"1"` (1st of the month).
- Job body per daily tick:
  1. If `monthly_digest_enabled != "true"` (new global on/off `AppSetting`, default `"true"`),
     return "disabled".
  2. If `monthly_digest_last_handled_month == currentMonth`, return "already sent this month".
     **Manual "run now" is subject to this gate too** — it's a no-op with the same status
     message once the month has been handled. Force-resending within a month (e.g. to reach
     new members) is out of scope; needs the per-recipient delivery ledger the spec parks as
     a non-goal.
  3. If `today.Day() < sendDay`, return "waiting for send day".
  4. Build content for the previous full calendar month, send (see below), then
     `UpsertSetting("monthly_digest_last_handled_month", currentMonth)` **whether or not** any
     email was actually sent (spec: "the month is still marked handled so the job doesn't keep
     checking daily for the rest of it").
- If `sendDay` > days-in-month (e.g. 31 in Feb), interpret as "last day of month" — small helper
  `min(sendDay, daysInMonth(today))` keeps this a config-doesn't-need-validation choice.
- Manual "run now" from the admin Jobs page bypasses step 3 only (spec: "A manual run bypasses
  the send-day gate but still marks the month handled"). Step 2 still applies per the note
  above.

### 7. Timezone: server local, not UTC

All "what month is it," "what day of the month," and "the previous full calendar month"
computations use plain `time.Now()` (which reads the process's local timezone), **not**
`time.Now().UTC()`.

- Rationale: "the 1st of every month" needs to mean the 1st in the community's local time to
  feel natural. A single-region self-hosted deployment (this app) sets `TZ` via the container
  env / Docker `TZ` value, and that's the single source of truth.
- Ripple: `ListCreatedBetween(from, to)` in Slice 3 is called with `from`/`to` computed as
  "the first instant of the previous local month" and "the first instant of the current local
  month" respectively. SQLite compares timestamps lexicographically on the string representation
  GORM writes, which is UTC-ISO8601 (`2026-01-01T00:00:00Z`) — so the digest service converts
  its local-timezone bounds to UTC before passing them to the query. Practically:
  `from = time.Date(y, m-1, 1, 0, 0, 0, 0, time.Local)`, `to = from.AddDate(0, 1, 0)`, pass
  both directly to GORM (which handles the local→UTC conversion at bind time).
- Ops implication worth naming in the Slice 3 PR: changing the container `TZ` mid-month could
  in principle skip or duplicate a send. Not worth engineering around; document as a "don't
  do that during the send window" ops note.

## Sliced by the spec's Implementation order

Each slice is a single PR sized to land independently green through CI. Tests are written first
per the repo's TDD rule; every slice ends with `pnpm nx affected -t lint test e2e` clean.

---

### Slice 1 — `MonthlyDigestEnabled` preference plumbing

**Goal:** a member can toggle the digest on/off from their profile and it round-trips through
`PATCH /auth/me`. No sending yet.

**Backend files touched:**

- `internal/db/migrations/000016_add_monthly_digest_enabled.{up,down}.sql` — new column,
  `NOT NULL DEFAULT 1`. Mirrors `000006`'s shape for `email_notifications_enabled`.
- `internal/models/models.go` — `User.MonthlyDigestEnabled bool` with
  `gorm:"column:monthly_digest_enabled;not null"` (no default tag — same zero-value-vs-unset
  reason `EmailNotificationsEnabled` documents).
- `internal/handlers/auth.go` — `finalizeRegistration()` and `setup()` set
  `MonthlyDigestEnabled: true` explicitly; `updateMeBody` gains `MonthlyDigestEnabled *bool`;
  `applyContactPrefsUpdate()` handles it.

**Frontend files touched:**

- `src/lib/types.ts` — `User.monthly_digest_enabled: boolean`.
- `src/lib/api.ts` — `updateMe()` accepts `monthly_digest_enabled`.
- `src/components/ProfileForm.tsx` — new `Switch` under the existing "Email notifications" one;
  copy: "Monthly community digest — a once-a-month summary of new books and top recommended
  books in the community."

**Tests (TDD, write first):**

- `internal/handlers/auth_test.go`:
  - `TestUpdateMe_MonthlyDigestEnabled` — PATCH with `{monthly_digest_enabled:false}`; GET /me
    reflects it; PATCH back to `true` works.
  - `TestFinalizeRegistration_DefaultsMonthlyDigestOn` — a freshly registered user has
    `MonthlyDigestEnabled=true`.
- `apps/bookshelf-e2e`: extend the profile spec — toggle persists across reload and the
  network PATCH carries the flag.

**Acceptance:** migration runs on a seeded DB; existing users' flag defaults to true; profile
toggle round-trips.

---

### Slice 2 — Unsubscribe token, endpoint, and confirmation page

**Goal:** clicking a signed unsubscribe link (that doesn't exist yet — Slice 3 mints it) flips
`MonthlyDigestEnabled=false` without a login and lands the member on a confirmation page.

**Backend files touched:**

- `internal/handlers/otp_link_token.go` — new `unsubscribeTokenTTL`, new purpose constant, new
  `issueUnsubscribeToken(userID)` / `verifyUnsubscribeToken(token) (userID, error)` sharing the
  existing HMAC signer.
- `internal/handlers/auth.go` (or a new small `internal/handlers/unsubscribe.go` — leaning
  toward the latter to keep `auth.go` from growing further; it's already near the cognitive-
  complexity ceiling): `POST /unsubscribe/digest` accepting `{token}`. Public, no auth. Verifies
  token, flips flag via `users.Save`, returns `{email}` (for the confirmation page copy) or 200
  with no body if the flag was already off (idempotent).
- Route registration in `cmd/server/main.go` mounts the new handler.

**Frontend files touched:**

- `src/app/(auth)/unsubscribe/page.tsx` — new page, mirrors the `?resetToken=` pattern in
  `src/app/(auth)/forgot-password/page.tsx`: reads `?token=`, calls
  `api.unsubscribeDigest({token})`, shows a plain confirmation ("You've been unsubscribed from
  the monthly digest. You can re-enable it any time from your profile settings.") or an error
  (invalid/expired token).
- `src/lib/api.ts` — new `unsubscribeDigest`.

**Tests (TDD, write first):**

- `internal/handlers/unsubscribe_test.go`:
  - valid token → 200 + flag flipped;
  - expired token → 400 (mint one with `exp` in the past via the helper);
  - token minted with a different purpose (e.g. `email_change`) → 400;
  - already-unsubscribed member → 200 (idempotent), flag stays false;
  - unknown user_id (deleted member) → 404.
- `apps/bookshelf-e2e`: mint a token in a test-only helper (server-side), open
  `/unsubscribe?token=...`, assert confirmation copy and that the profile toggle reflects the
  new state after login.

**Acceptance:** unsubscribe endpoint works standalone; confirmation page renders both success
and error states.

---

### Slice 3 — Digest service, repo queries, scheduler wiring, admin settings, send loop, housekeeping

**Goal:** on the configured day of the month, opted-in members receive one email covering the
previous full calendar month; the admin Jobs page picks up the job with "run now."

This is the "actually build the feature" slice — every remaining piece lands in one PR.
Repo queries live with their sole caller (easier to review than in isolation), and the
housekeeping doc edits go with the feature they describe (otherwise `TODO.md` contradicts
reality until a follow-up PR ships).

**Backend files touched:**

_New repo methods:_

- `internal/repository/gorm/book_repo.go` — new `ListCreatedBetween(from, to time.Time, limit
int)`, preloads `Copies`.
- `internal/repository/gorm/recommendation_repo.go` — new `ListTopBooks(limit int)` returning
  `[]TopRecommendedBook{Book, Count}`, ordered `count DESC, title ASC`, excludes zero-count.
- `internal/repository/gorm/user_repo.go` — new `ListDigestRecipients(ctx)` filtering on
  verified/not-suspended/not-pending/opted-in.
- Interfaces in `internal/repository/interfaces.go` (or wherever the app declares them; adjust
  during implementation) updated so the digest service can inject.

_Digest service and wiring:_

- `internal/services/digest.go` (new) — `DigestService{books, recommendations, users, admin,
email, cfg, clock}`. Methods:
  - `Run(ctx) string` — the `RegisterJob` callback. Returns the human-readable status string
    the admin Jobs page displays.
  - `assembleContent(from, to) (DigestContent, error)` — pulls the two sections, returns
    zero-value struct if both empty.
  - `render(recipient, content) (subject, html string)` — uses the existing `email.URL` /
    `email.Button` helpers plus new `email.UnsubscribeLink(userID)`.
  - `sendAll(ctx, recipients, content) (sent, failed int)` — plain sequential loop calling
    `email.SendEmail` per recipient; logs and continues past per-send errors; returns
    aggregated counts.
- `internal/services/email.go` — add `UnsubscribeLink(userID uint) string` wrapping
  `issueUnsubscribeToken` + `URL("/unsubscribe?token=...")`.
- `internal/services/scheduler.go` — no shape change needed; the job is registered in
  `cmd/server/main.go` via the existing `RegisterJob` API, interval defaulting from a new
  `AppSetting` key `monthly_digest_interval` (default `24h` — the daily tick).
- `internal/db/db.go` `Seed()` — seed the four new settings:
  - `monthly_digest_enabled = "true"` (global on/off)
  - `monthly_digest_send_day = "1"`
  - `monthly_digest_new_books_limit = "10"`
  - `monthly_digest_top_recommended_limit = "5"`
  - (`monthly_digest_last_handled_month` is not seeded — created by first successful run.)
- `cmd/server/main.go` — construct `DigestService`, `scheduler.RegisterJob("monthly-digest",
"monthly_digest_interval", 24*time.Hour, digest.Run)`.
- `internal/handlers/jobs.go` — new `POST /admin/jobs/monthly-digest/test-email` alongside
  the existing `POST /admin/jobs/{job}/run`. Auth-required (admin only). Calls
  `digest.SendTestEmail(ctx, adminUser)` which: assembles content for the previous full
  calendar month (same as a real run), renders it for the calling admin's email address, sends
  one email to that address, and returns `{sent: true, recipient: "admin@..."}`. Bypasses all
  gating (enabled flag, send-day, once-per-month idempotency) and **does not mark the month
  handled** — it's a preview, not a real run. The unsubscribe link in the test email is real
  (minted token, fully functional) so the template renders exactly as recipients will see it.

**Frontend files touched:**

- `src/app/admin/jobs/page.tsx` — extend `JOB_SETTING_KEYS` so the "monthly-digest" row shows
  its interval setting and, optionally, the four content/config settings inline (matches how
  other jobs are already surfaced). Add a "Send test email" button beside the existing "Run
  now" button — calls the new test endpoint and shows a brief toast ("Test email sent to
  admin@...").

**Housekeeping (same PR):**

- `apps/bookshelf/TODO.md` — remove the "digest email" entry from the "Someday / hold" list.
- `apps/bookshelf/CLAUDE.md` — no scope-guardrail amendment (spec calls this out); optional:
  add a one-liner under an existing section pointing to this spec/plan if a natural home
  exists.
- `apps/bookshelf-backend/CLAUDE.md` — add a short note under "Known gaps" restating the three
  non-goals the spec explicitly parks: no per-recipient delivery ledger, no daily-budget
  tracking (the 300/day relay cap is a non-issue at ~100-member scale), and no worker-pool /
  overflow handling. Matches how that file already documents intentionally-parked risks.

**Tests (TDD, write first):**

_Repo-level:_

- `book_repo_test.go` — seed 5 books across 3 months, assert `ListCreatedBetween` returns only
  the middle month's, newest first, respects `limit`.
- `recommendation_repo_test.go` — `ListTopBooks` orders by count desc, title asc; excludes
  zero-count books; respects `limit`; handles zero recommendations across the whole DB
  (empty slice, no error).
- `user_repo_test.go` — matrix over the four filter dimensions: seed one user per (verified,
  suspended, pending, opted-in) combination; assert `ListDigestRecipients` returns exactly the
  all-true combo.

_Service-level (`internal/services/digest_test.go`):_

- **Gating:** `Run` returns "disabled" when `monthly_digest_enabled=false`; returns "already
  sent" when `monthly_digest_last_handled_month == currentMonth` (applies to both scheduled
  and manual "run now" invocations — see design decision 6); returns "waiting for send day"
  when `today.Day() < sendDay`.
- **Empty period:** both sections empty → no email sent, month marked handled.
- **Content assembly:** given seeded books and recommendations, the returned `DigestContent`
  has the right items per section, respects per-section limits, applies the count-desc/
  title-asc tie-break.
- **Manual run bypasses send-day (but not once-per-month):** invoked with `today.Day() <
sendDay` on a fresh month, still runs and marks the month handled; a second manual run in
  the same month is a no-op.
- **Send loop tolerates per-recipient failure:** with a fake `EmailService` that errors on
  the 2nd of 5 recipients, the other 4 still receive the digest, result string reports
  "sent 4, failed 1", month is still marked handled.
- **Idempotency within a month:** two `Run` calls back-to-back on the same day → second is a
  no-op.
- **Test email:** `SendTestEmail` sends exactly one email to the given admin, returns the
  recipient address; does not flip `monthly_digest_last_handled_month`; works even when
  `monthly_digest_enabled=false` and even mid-month after a real run has already been marked
  handled.

_E2E:_ extend the admin-jobs spec to trigger the digest via "run now" and assert the resulting
status string appears in the Jobs table; also click "Send test email" and assert the toast
appears (SMTP capture confirms one email received by the admin's address).

**Acceptance:** the manual "run now" produces the expected email against a real SMTP capture
(local dev's mailhog / whatever this repo uses today for e2e SMTP — check
`apps/bookshelf-e2e/CLAUDE.md`), gating logic behaves per the tests above, admin can flip the
global switch and change the send day without a redeploy, and the TODO.md entry is gone.

## Cross-cutting notes

- **No new admin UI framework.** Every configurable knob is a plain `AppSetting`, edited via
  the existing `PATCH /admin/settings`. Surface on the admin Jobs page via `JOB_SETTING_KEYS`
  the same way `cover_refresh_interval` and `backup_interval` are today.
- **CHANGELOG entries for each slice.** `nx release` picks these up on merge; all three
  slices are member-visible in some way (preference toggle, unsubscribe page, the digest
  itself), so use `feat` for all three.
- **Migration.** Only Slice 1 adds one. The default-true column value is set both at the SQL
  layer (for existing rows via `DEFAULT 1`) and in Go at user-creation time (for the
  zero-value-vs-unset reason `EmailNotificationsEnabled` already documents).
- **What a manual e2e sanity-check looks like end-to-end** (after Slice 3): seed a few books
  dated in the previous month + one recommendation, admin clicks "run now", inbox receives the
  digest, click the footer link, land on the confirmation page, verify the profile toggle now
  reads off. Fold this into the e2e suite rather than keeping it as a one-off checklist, per
  the repo-root `CLAUDE.md`'s "Verifying changes" rule.

## Open questions worth flagging before Slice 3

1. **`FRONTEND_ORIGIN` correctness in production** — the unsubscribe link must be the real
   public URL, not a container-internal one. Verify in the Slice 2 PR against
   `compose/docker-compose.bookshelf.yml`'s `FRONTEND_ORIGIN` value before merging Slice 3.
2. **Subject line and From address** — deliberately deferred to the Slice 3 PR. Proposed
   default: subject `Bookshelf — <Month> digest` (e.g. "Bookshelf — August digest"), From
   address reuses the app's existing `SMTP_FROM`/whatever `EmailService` already sends every
   other mail as (no new env var). Flag in the PR for review; happy to iterate on copy there.
