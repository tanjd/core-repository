# Monthly digest email — spec

**Status:** Implemented (#84) · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `User`, `Book`, `Recommendation`, `EmailService`, `Scheduler`, `AppSetting`

Once a month, send opted-in members a short email covering what's new in the community:
books recently added to the catalog and the current top-recommended books. Members control
this independently of the existing transactional email toggle, and every email carries a
one-click way to stop receiving it.

## Why now

- Members have no periodic "what happened this month" surface today — the catalog and
  changelog exist, but nobody proactively hears about them unless they visit the app.
- The SMTP relay this app sends through caps at **300 emails/day**, shared across every kind
  of mail this app sends. The community's realistic ceiling is ~100 members, so a monthly
  digest — even on the one day of the month it fires — comes nowhere near that budget when
  added to the day's transactional traffic. The design below treats the cap as a real-world
  non-issue at this scale and defers any budget-tracking machinery until it stops being one.

## Goals

- Once a month, opted-in members get one email covering: books added to the catalog that
  month, and the current top-recommended books.
- Every member can opt in/out from profile settings; the digest defaults **on**.
- Every digest email carries a one-click, no-login unsubscribe link.
- A month with nothing to report (no new books and an unchanged top-recommended list) sends
  **no email** — there's no such thing as an empty digest.
- Send day, per-section item limits, and a global on/off switch are admin-configurable via
  the existing `AppSetting` mechanism, surfaced on the existing admin Jobs/Settings pages —
  no new admin UI framework.

## Non-goals (v1)

- **Per-member content personalization** (e.g. "books like ones you've borrowed"). This is
  a community-wide digest, not a recommendation engine — keeps scope aligned with
  `apps/bookshelf-backend/CLAUDE.md`'s "not a book discovery product" guardrail.
- **An in-app (bell) notification for the digest.** Email-only. There's nothing here that
  isn't already reachable from the catalog/changelog pages, so a new `Notification` type
  would add surface without a clear benefit.
- **A shared daily send-budget counter, worker pool, or overflow handling.** With a
  community ceiling of ~100 members and a per-run cost of "one email per opted-in recipient
  once a month," the SMTP relay's 300/day cap isn't a live constraint. v1 sends one message
  per recipient, sequentially, using the same `EmailService.SendEmail` every other mail in
  the app already goes through. If the relay ever pushes back (rate-limit response, transient
  5xx), the individual send returns an error, the digest job logs how many recipients were
  reached before the failure, and the admin sees that on the Jobs page. Revisit budget
  tracking and/or a worker pool only if the community grows enough — or the relay gets
  chattier — that this posture stops working.
- **A per-recipient delivery ledger or retry-safe resumability.** A crash mid-send could, in
  principle, cause a partial resend on the next daily check before that month is marked
  done. Accepted risk for a single-instance, self-hosted app where mid-send crashes are
  rare — not worth a new delivery-tracking table for v1. Named here rather than silently
  ignored, matching this repo's habit of documenting known gaps (see
  `apps/bookshelf-backend/CLAUDE.md`'s "Known gaps" section for the pattern).
- **A "what's new" / changelog section.** Members can already reach `/changelog` from the app,
  and surfacing it in the digest would require new backend↔frontend plumbing (the backend owns
  scheduling and sending but has no access to the frontend's build-time-parsed changelog).
  Deferred until there's demand; the two remaining sections already cover "what's new in the
  community" without it.

## Audience and opt-in/out

Eligible recipients are members who are verified, not suspended, not pending approval, and
have the digest enabled. This is a **new, separate flag** from `EmailNotificationsEnabled`
— that field's doc comment and profile-page copy already scope it specifically to
transactional loan/wishlist mail ("account/security emails are unaffected"), so overloading
it here would silently change what it means to existing users.

- `User.MonthlyDigestEnabled` defaults **true** for every member, existing and new — same
  "opt-out" posture as `EmailNotificationsEnabled` today.
- Toggled from profile settings, next to the existing "Email notifications" switch, with
  copy naming what the digest contains and its monthly cadence.
- **One-click unsubscribe.** Every digest email's footer links to a token-bearing URL that
  flips the flag off without requiring a login — standard practice for any periodic/bulk
  mail, and the only way a member who never logs in again can stop it. The link needs its
  own longer-lived signed token, distinct from this app's existing 15-minute magic-link
  tokens (registration/reset/email-change), since a digest might not be opened for weeks.
  It carries only the member's identity — no separate code — because there's nothing to
  confirm beyond "this link is genuine."

## Scheduling

This app's scheduled jobs (backup, cover-refresh, cover-backfill, ...) all run on a fixed
interval from their last run — fine for "every 24 hours," but a naive 30-day interval
drifts against calendar months and stops meaning "the 1st of every month." The digest job
instead ticks daily and is idempotent per calendar month:

- On each daily tick, it checks whether the current month has already been handled. If so,
  it does nothing.
- Otherwise, once the configured send day of the month has arrived, it builds and sends the
  digest for the **previous full calendar month**, then marks the current month handled so
  it doesn't re-check for the rest of it.
- A global on/off switch lets an admin disable the whole feature without touching the
  scheduler or redeploying.
- The admin Jobs page picks this job up the same way it does every other job — including
  its existing manual "run now," which is useful for testing or forcing a resend. A manual
  run bypasses the send-day gate but still marks the month handled, so the automatic tick
  doesn't send a second copy later that month.

## Content

Two sections, each covering the previous full calendar month, each admin-configurable in how
many items it shows, and each omitted entirely from the email when empty (no "nothing new
this month" placeholders):

1. **New books** — books added to the catalog during the period, newest first, capped at a
   configurable limit (suggested default: 10).
2. **Top recommended books** — the current most-recommended books community-wide (not just
   ones recommended during the period), using the same thumbs-up counts and tie-break
   (ties resolve by title) as the catalog's existing "Most Recommended" sort. Capped at a
   configurable limit (suggested default: 5).

If both sections are empty for the period, nothing is sent that month, and the month is
still marked handled so the job doesn't keep checking daily for the rest of it.

## Send loop

Iterate opted-in recipients and call the existing `EmailService.SendEmail` per member,
sequentially. No worker pool, no concurrency, no daily-budget bookkeeping (see "Non-goals").
Per-send errors are logged individually; the job's result string surfaces "sent N, failed M"
on the admin Jobs page. A run continues past individual failures — one member's SMTP-side
rejection shouldn't cost every subsequent member their digest — and the month is still marked
handled at the end regardless of how many sends succeeded (spec: no per-recipient delivery
ledger, no retry-safe resumability in v1).

## Email content and format

Same house style as every other email this app sends: server-rendered HTML built per send,
reusing the existing helpers for building links and call-to-action buttons back into the
app. Section order: new books, then top recommended. Footer includes the one-click
unsubscribe link plus a note that the same preference lives in profile settings.

## Backend changes

| Area                | Change                                                                                                                                                                                                                                                                                                        |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `User` model        | New `MonthlyDigestEnabled bool` column, `NOT NULL DEFAULT true`, following `EmailNotificationsEnabled`'s existing pattern (including setting it explicitly `true` at register/setup time rather than relying on a GORM default tag, for the same zero-value-vs-unset reason already documented on that field) |
| Migration           | One new migration pair adding the column                                                                                                                                                                                                                                                                      |
| `PATCH /me`         | Accepts an optional `MonthlyDigestEnabled` flag, mirroring how `EmailNotificationsEnabled` is already updated                                                                                                                                                                                                 |
| Magic-link tokens   | A new, long-lived (e.g. ~1 year) signed-token purpose for the unsubscribe link, separate from the existing short-lived OTP-link tokens — same signing mechanism, different purpose and lifetime                                                                                                               |
| New public endpoint | Verifies an unsubscribe token and flips the flag off — no auth required, since the whole point is working without a login                                                                                                                                                                                     |
| Book queries        | A new query for books created within a date range                                                                                                                                                                                                                                                             |
| Recipient query     | A query for verified/active/opted-in members                                                                                                                                                                                                                                                                  |
| New digest service  | Owns content assembly, a simple sequential send loop, and the calendar-aware monthly gating described above                                                                                                                                                                                                   |
| Scheduler wiring    | Registers the digest job alongside the existing jobs (backup, cover-refresh, etc.), using the same `RegisterJob` mechanism                                                                                                                                                                                    |

## Frontend changes

| Area                 | Change                                                                                              |
| -------------------- | --------------------------------------------------------------------------------------------------- |
| Profile settings     | A new "Monthly email digest" toggle alongside the existing "Email notifications" one                |
| New unsubscribe page | Reads the token from the link, calls the backend's unsubscribe endpoint, shows a plain confirmation |

No `CLAUDE.md` product-scope guardrail amendment is needed here, unlike the recommendations
feature — this digest only summarizes data that's already public elsewhere in the app
(catalog, changelog); it doesn't introduce new member-provided metadata.

## Housekeeping

Shipped in #84. `apps/bookshelf/TODO.md`'s "Someday / hold" entry for this feature has been
removed accordingly, so the file no longer contradicts a feature that's actually built.

## Testing

- **Unit:** content assembly for each section (date-range filtering, top-N ordering and tie
  -break, empty-period behavior) and the calendar-aware monthly gating (send-day check,
  once-per-month idempotency).
- **Backend:** toggling the new preference via `PATCH /me`; the unsubscribe endpoint against
  a valid token, an expired token, a token for the wrong purpose, and an already-unsubscribed
  member.
- **E2E (`apps/bookshelf-e2e`):** the profile toggle persists and is reflected back by the
  API; the unsubscribe link flow end-to-end from a minted token through to the flag flipping.

## Resolved decisions

- **Default state:** opt-out (digest defaults on for every member) rather than opt-in —
  maximizes initial reach; the unsubscribe link and profile toggle both exist to make
  opting out easy.
- **No "what's new" / changelog section in v1:** members can already reach `/changelog` from
  the app, and pulling changelog entries into the digest would require net-new backend↔frontend
  plumbing (backend owns sending but the changelog is parsed only in the frontend build).
  Deferred; the two content sections that remain already cover "what's new in the community."
- **300/day cap:** treated as a real-world non-issue at ~100 members. No budget tracking, no
  worker pool, no overflow handling — send sequentially, log per-send failures, deal with it
  if the relay ever pushes back. Revisit when membership or transactional volume actually
  gets close.
- **Unsubscribe:** a one-click, no-login link in the email itself, in addition to the
  profile toggle — standard practice for periodic mail, and necessary for members who won't
  log back in just to stop it.

## Implementation order

1. `MonthlyDigestEnabled` column + migration + `PATCH /me` + profile toggle (smallest
   independently-shippable slice).
2. Unsubscribe token purpose + unsubscribe endpoint + unsubscribe page (works standalone
   with a manually-minted token before the digest job exists).
3. The digest itself — new-books/eligible-recipients repo queries + digest service (content
   assembly, sequential send loop, monthly gating) + scheduler wiring + admin-configurable
   settings + stale `TODO.md` entry removed.
