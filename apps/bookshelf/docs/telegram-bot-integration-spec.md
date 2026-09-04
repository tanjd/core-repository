# Telegram bot integration — spec

**Status:** Proposed (not yet implemented) · **Scope:** `apps/bookshelf` +
`apps/bookshelf-backend` + new `apps/bookshelf-bot` · **Depends on:** `User`, `LoanRequest`,
`Notification`, `EmailService`, `Scheduler`, `AppSetting`, `telegram-bot-shared`

Give bookshelf members real-time push notifications (loan requests, approvals, wishlist
fulfillment, due-date reminders) via a Telegram bot, as a third channel alongside the existing
in-app `Notification` bell and transactional email.

## Why

Members who don't check the web app often miss loan requests, approvals, and wishlist
fulfillments until they happen to log in. Telegram closes that gap — real-time delivery to a
channel most members already have open, without asking them to install or monitor a new
surface.

This repo already has three Telegram bots (`index-watch`, `otobr-buddy`, `table-talks`), but
all three are fully standalone — local SQLite, no calls into a backend API. This is the
**first integration between a Telegram bot and an existing Go backend service** in this repo,
so a few conventions the other bots follow don't carry over cleanly; those deviations are
called out explicitly below.

## Goals (v1)

- A member links their Telegram account to their bookshelf `User` via a one-tap deep link from
  the web app.
- Once linked, every event that already creates an in-app `Notification` (loan request
  received, request accepted/rejected, copy transferred, wishlist fulfilled) also pushes a
  Telegram message. Linking Telegram **is** the opt-in — no separate per-type preference UI in
  v1, mirroring how `EmailNotificationsEnabled` is a single on/off today, not per-type.
- A new scheduled job reminds borrowers N days before a loan's `expected_return_date`,
  alongside the existing cover-refresh/backup/digest jobs on `Scheduler`.

## Non-goals (v1)

- No inline "quick action" buttons (approve/reject a loan request, mark a copy returned) from
  Telegram — deferred to v2, see below.
- No catalog search / `/find` command from the bot.
- No per-notification-type Telegram preferences.
- No group-chat mode — one bot chat per linked member, same as email today.

## Architecture decision: backend calls Telegram directly

`bookshelf-backend` gains a `TelegramService` (`internal/services/telegram.go`) that POSTs
straight to `api.telegram.org` using the bot token — **not** a call into the bot's own HTTP
API. This deliberately deviates from "the bot service owns all Telegram traffic":

- Mirrors the existing pattern exactly: `EmailService` already sends SMTP directly from the
  backend, no intermediary service. Telegram becomes another notification channel in the same
  place "who to notify and how" already lives, not a new inter-service dependency.
- Delivery survives a bot-container restart/outage — only inbound commands (`/start` for
  linking) are briefly unavailable, matching how email and in-app notifications are already
  independent of each other.
- No new internal auth surface needs to be built and secured for v1 beyond the one linking
  endpoint (see below).

Consequence: the `telegram_id`/`chat_id` link lives in **bookshelf-backend's own database**
(new nullable columns on `User`), not in the bot's local SQLite the way `otobr-buddy` et al.
do — because the backend, not the bot, drives outbound delivery. The bot process still runs
its own lightweight long-polling loop (like the other three bots), but only to handle the
inbound `/start` linking command; it holds no notification state of its own.

Follow `SMSService`'s existing interface-plus-mock convention (`internal/services/sms.go`) for
testability: a narrow `TelegramNotifier` interface, real implementation injected in
`cmd/server/main.go`, fake used in workflow tests — same narrow-interface-for-testing shape
`digestEmailer` already uses in `digest.go`.

## Data model changes

Add to `User` (`internal/models/models.go`), migration `000020_add_telegram_link`:

```go
TelegramChatID   *int64     `gorm:"column:telegram_chat_id;uniqueIndex" json:"-"`
TelegramLinkedAt *time.Time `gorm:"column:telegram_linked_at" json:"telegram_linked_at,omitempty"`
```

`TelegramChatID == nil` means "not linked" — the opt-in gate, checked the same way
`bookCopy.Owner.EmailNotificationsEnabled` gates email sends today (see `loan_workflow.go`
`OnRequested`). `json:"-"` on the chat ID itself — no reason to expose it to the frontend
beyond a linked boolean; add a computed `telegram_linked bool` to whatever profile response the
frontend already reads.

Add to `LoanRequest`: `DueReminderSentAt *time.Time`, for the due-date-reminder job's dedupe
(same migration, or a follow-up pair).

## Linking flow (deep link from web app)

1. Bookshelf frontend adds a "Connect Telegram" button (profile/settings, alongside the
   existing email-notification toggle).
2. Backend gains `POST /profile/telegram/link-token` (authenticated like every other
   `/profile`-ish endpoint) — issues a short-lived, single-use signed token (reuse the
   existing `golang-jwt/jwt/v5` + `jwtSecret` already wired into `EmailService`/auth handlers,
   not a new secret) encoding the `User.ID`.
3. Button opens `https://t.me/<bot_username>?start=<token>`.
4. Bot's `/start <token>` handler calls a new backend endpoint,
   `POST /internal/telegram/confirm-link` (bot→backend, not user-facing — protected by a
   static shared-secret env var, the one new bot↔backend auth surface v1 needs), with the token
   and the Telegram `chat_id` it received. Backend verifies the token, sets
   `User.TelegramChatID`/`TelegramLinkedAt`, returns a display name so the bot can reply
   "Linked to <name>'s bookshelf account ✅".
5. Token expiry (~10 minutes) and single-use (mark consumed, or rely on JWT `exp` plus
   deleting/ignoring after first use) prevent replay.

An "unlink" affordance (button in bookshelf settings → `DELETE /profile/telegram/link`, clears
both columns) ships alongside link — cheap, and this kind of account link always needs a way
back out.

## Notification hook points

`NotificationRepository.Create` is called at ~8 sites across `loan_workflow.go` and
`wishlist_workflow.go`, each already immediately followed by an inline, best-effort
`EmailService.SendEmailAsync(...)` call built from context the workflow already has in hand
(borrower name, book title, etc. — see `OnRequested` for the shape).

A generic "wrap `NotificationRepository.Create` and synthesize a message from the bare
`Notification` row" approach was considered and rejected: the `Notification` model only
carries a `Type` string and FK IDs, not human-readable content — that content is assembled
per-call-site today specifically for the email body, with no reasonable way to reconstruct it
centrally without re-fetching everything the workflow already had loaded.

**Chosen approach**: add a `telegram TelegramNotifier` field to `LoanWorkflow` and
`WishlistWorkflow` (same constructor-injection shape as `email`), and add one
`w.telegram.NotifyAsync(ctx, recipient.TelegramChatID, text)` call directly beside each
existing `SendEmailAsync` call, reusing the same already-assembled context. Telegram uses
`parse_mode: HTML` with a subset of tags, so the existing `html.EscapeString`-built bodies
mostly carry over, minus the `Button()` link becoming a plain inline URL. More call sites
touched than a single wrapper, but it's the consistent continuation of how this codebase
already does per-event notification fan-out, and keeps each notification's content colocated
with the workflow logic that knows what happened.

Every existing `Notification` `Type` gets a Telegram counterpart: touch every site currently
calling `SendEmailAsync`, gated on `recipient.TelegramChatID != nil` the same way email is
gated on `EmailNotificationsEnabled`. Failures are logged and swallowed
(`zerolog.Ctx(ctx).Warn()`), never fail the underlying operation — same contract as email
today.

> **Note:** [`telegram-notification-preferences-spec.md`](./telegram-notification-preferences-spec.md)
> extends this gate to `TelegramChatID != nil && TelegramNotificationsEnabled` once a
> member can independently toggle Telegram notifications off without unlinking — read it
> alongside this section before implementing the guard.

**Extended beyond `LoanWorkflow`/`WishlistWorkflow` post-implementation**: an admin is a
`User` like any other, so `RegistrationWorkflow.OnPendingApproval` (emails every admin when a
new registration needs approval) got the same `telegram TelegramNotifier` field and one
`NotifyAsync` call beside its existing `SendEmailAsync`, gated identically
(`TelegramChatID != nil && TelegramNotificationsEnabled`) — no separate "admin notification
channel" concept needed, since the same per-user Telegram link/toggle already covers it.
`OnApproved`/`OnRegistered` (user-directed welcome emails, not admin-directed) were left
untouched — out of scope for "admin notifications."

## Due-date reminder job

New `RegisterJob` entry on `Scheduler`, following the existing cover-refresh/backup job shape
(interval via a new `AppSetting` key, e.g. `due_date_reminder_interval` default `24h`,
`lastRunAt` persisted via `AdminRepository` the same way `coverRefreshLastRunKey` is). Each
run:

1. Query active loans (`LoanRequestRepository`) whose `expected_return_date` is exactly N days
   out (`due_date_reminder_days_before` `AppSetting`, default `2`).
2. For each, create an in-app `Notification` (`loan_due_soon`) unconditionally, email if
   `EmailNotificationsEnabled`, and push to Telegram if linked and enabled — the same
   three-way fan-out every other event in this app uses (see `loan_workflow.go`).
3. Check/set `LoanRequest.DueReminderSentAt` to avoid duplicate reminders on subsequent runs.

**Revised after initial ship**: this originally shipped Telegram-only (no in-app row, no
email), reasoned as "new capability, not backfilling an existing channel." That created a real
inconsistency — every other event in the app notifies in-app unconditionally and lets the
member choose channels via preferences, but due-date reminders were Telegram-only with no
opt-out short of unlinking. Brought in line with the rest once that stood out: in-app is now
unconditional (matches how every other `Notification`-creating event behaves), and email/
Telegram both follow the member's own existing per-channel toggles rather than the event having
its own bespoke channel rule.

## Bot app (`apps/bookshelf-bot`)

Scaffold via `make new-bot NAME=bookshelf-bot` (existing generator) — `telegram-bot-shared` for
health-check server, dev/prod token selection, logging setup. Deployed independently (own
Dockerfile, own `nx release` versioning, published to `ghcr.io/tanjd/bookshelf-bot`), matching
every other app in this repo.

- **Responsibilities are intentionally thin for v1**: handle `/start <token>` (call backend's
  `confirm-link` endpoint, reply with confirmation), a `/help`/`/status` pair for basic
  usability. No local database needed — unlike `otobr-buddy`/`index-watch`, this bot holds no
  state; the backend is the source of truth for the link.
- `python-telegram-bot>=21.0`, long-polling via `app.run_polling(...)`, same as every other bot
  in this repo — no reason to introduce webhooks here.
- New env vars: `BOOKSHELF_BACKEND_URL` and a shared internal-auth secret
  (`BOOKSHELF_INTERNAL_TOKEN`, matched against what `bookshelf-backend` expects on
  `/internal/telegram/confirm-link`) — the one new bot↔backend coupling, scoped to exactly one
  endpoint.

## Deferred to v2

- Inline quick actions (approve/reject a loan request, mark a copy returned) from Telegram
  message buttons. Open question when picked up: bot→backend call auth for _acting as a
  user_ — a service-account JWT (bot resolves `telegram_id` → `User`, backend authorizes the
  call as that user) vs. a per-user token minted at link time and stored by the bot.
- `/find <title>` catalog search from the bot.
- Per-notification-type Telegram preferences.
- **Overdue-loan notification.** `DueReminderService` only fires once, before the due date
  (`due_date_reminder_days_before`) — nothing currently notifies anyone once a loan is
  actually overdue (`expected_return_date` passed, not yet returned). "Overdue" today is only
  a passive badge (`OverdueCount` dashboard stat, frontend status badges) — nobody is
  proactively told. When picked up: a second scheduler job (or an extension of
  `DueReminderService`) querying accepted loans past `expected_return_date` with
  `returned_at IS NULL`, notifying **both** the borrower and the lender (unlike the pre-due
  reminder, which is borrower-only) — same in-app-always, email/Telegram-per-preference
  fan-out. Needs its own dedupe marker (a new `LoanRequest` column, e.g.
  `OverdueNotifiedAt`) so it doesn't nag on every scheduler tick once triggered.

## Files to touch (representative, not exhaustive)

- `internal/models/models.go` — `User.TelegramChatID`/`TelegramLinkedAt`,
  `LoanRequest.DueReminderSentAt`.
- `internal/db/migrations/000020_add_telegram_link.{up,down}.sql` (+ a pair for
  `DueReminderSentAt`, or folded into the same migration).
- `internal/services/telegram.go` (new) — `TelegramNotifier` interface + real implementation,
  following `sms.go`'s interface/mock shape and `email.go`'s HTTP-send shape.
- `internal/services/loan_workflow.go`, `internal/services/wishlist_workflow.go` — inject
  `telegram`, add `NotifyAsync` calls beside existing `SendEmailAsync` calls.
- `internal/services/scheduler.go` — new due-date-reminder `RegisterJob`.
- `internal/handlers/` — new handler for `POST /profile/telegram/link-token`,
  `DELETE /profile/telegram/link`, `POST /internal/telegram/confirm-link`.
- `cmd/server/main.go` — wire `TelegramService` into workflows and scheduler, same shape as
  existing `emailSvc`/`notifRepo` wiring.
- `apps/bookshelf/` frontend — "Connect Telegram" button + linked-status display in
  profile/settings.
- `apps/bookshelf-bot/` (new app, via generator) — `/start` linking handler, `/help`.

## Verification

- Backend: extend `loan_workflow_test.go`/`wishlist_workflow_test.go` with a fake
  `TelegramNotifier` (same shape as the existing `digestEmailer` fake) asserting the right
  message fires per event, gated correctly on `TelegramChatID` presence.
- Backend: new handler tests for link-token issuance/consumption/expiry and the internal
  confirm-link endpoint (reject bad/missing shared secret).
- Scheduler: unit test for the due-date-reminder job's dedupe, following `scheduler_test.go`'s
  existing job-testing pattern.
- Bot: manual verification — run `bookshelf-backend` + `bookshelf-bot` locally, click "Connect
  Telegram" in the dev frontend, confirm the deep link completes linking and a subsequent loan
  request produces a real Telegram message in a test chat. No `bookshelf-e2e` coverage exists
  yet for loan-request flows at all, so this stays manual for v1 rather than blocking on
  building that suite first.
