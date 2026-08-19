# Bookshelf — TODO and ideas

Covers both `apps/bookshelf` (frontend) and `apps/bookshelf-backend` (API) as one product.
The app is now live publicly (see `compose/docker-compose.bookshelf.yml` — Traefik +
Cloudflare, domain `bookshelf.tanjd.com`), superseding the old Tailscale-only internal
testing setup `apps/bookshelf-backend/docker-compose.example.yml` still documents.

## Security — from the 2026-08-18 audit

Full audit covered every route's auth/ownership check, JWT/crypto/email code, and the
production compose file. All findings except the JWT-storage trade-off below have since
been fixed.

**Fixed:**

- ~~HTML/email injection via unescaped `User.Name`/`Book.Title` in transactional emails~~ —
  `internal/services/loan_workflow.go` and `internal/handlers/auth.go` now run every
  user-controlled field through `html.EscapeString` before interpolating into the
  `text/html` email bodies. Was exploitable via a crafted display name or book title
  rendering as a live link/markup in emails sent to _other_ users (phishing vector).
- ~~Directory listing on `/covers/`~~ — `cmd/server/main.go`'s cover static handler now
  404s any request path ending in `/` instead of falling through to
  `http.FileServer`'s default listing.
- ~~No rate limiting on `/auth/login`, `/auth/register`, `/auth/send-otp`~~ — `/auth/login`
  already had its own email-keyed limiter (`internal/ratelimit`, `loginLimiter`, 5 attempts/
  15min, predates this audit); `/auth/register` and `/auth/send-otp` were fixed in #41 with a
  separate token-bucket limiter (`internal/middleware/ratelimit.go`; register keyed by IP at
  5 burst/10min refill, send-otp keyed by user ID at 3 burst/5min refill).
- ~~Backend (8000) and frontend (3000) ports published directly to the host~~ — both
  `ports:` blocks dropped from `compose/docker-compose.bookshelf.yml`; Traefik already
  routed to the frontend via the `proxy` network without them, and the backend was never
  reachable from outside the `bookshelf` network to begin with.
- ~~Possible SMTP header injection via the new-email field~~ — `EmailService.SendEmail`
  (`internal/services/email.go`) now rejects any `recipient`/`subject` containing `\r`/`\n`
  before building the raw MIME header block, unconditionally (even when SMTP delivery
  itself is a local no-op) — a single defense-in-depth guard at the one funnel point every
  email in the app goes through.
- ~~`/auth/setup` TOCTOU race~~ — `UserRepository.CreateAdminIfNoneExists` wraps the
  admin-existence check and the `Create` in one `db.Transaction` (same idiom as
  `LoanRequestRepository.CreateAndMarkRequested`), re-checking on the transactional handle
  and returning `ErrConflict` if an admin already exists. The handler keeps a cheap
  `HasAdmin()` fast-path check up front (avoids paying bcrypt cost-12 hashing on every hit
  to an already-closed endpoint) but the transaction is the authoritative guard closing the
  actual race window.
- ~~No CSP/HSTS/X-Frame-Options/X-Content-Type-Options anywhere~~ — headers are now split by
  layer to avoid duplicate/conflicting sources of truth: the backend gets
  `X-Content-Type-Options`, `X-Frame-Options`, and `Referrer-Policy` from a new Go middleware
  (`internal/middleware/security_headers.go`), since it has no Traefik labels of its own. The
  frontend route gets the same three headers plus HSTS from Traefik labels on
  `bookshelf-frontend` in `compose/docker-compose.bookshelf.yml` (`stsSeconds=31536000`,
  `stsIncludeSubdomains=true`, `stsPreload=false`, `contentTypeNosniff`,
  `customFrameOptionsValue=SAMEORIGIN`, `referrerPolicy`) — Traefik sits downstream of the
  origin in the response path, so it's the effective source of truth there regardless of
  what the app sets. `next.config.ts`'s `headers()` carries only the `Content-Security-Policy`
  (no Traefik equivalent), verified empirically against the production standalone build with
  Playwright across `/`, `/login`, `/setup`, `/catalog`, `/register` — `script-src` and
  `style-src` both needed `'unsafe-inline'` since App Router embeds per-page inline
  RSC-hydration `<script>` tags (a different hash every build) and Tailwind/Next inject
  inline `<style>`.

**Still open:**

- **JWT stored in `localStorage`** (`src/lib/api.ts`) rather than an httpOnly cookie.
  Standard trade-off for a Bearer-token SPA (and it buys CSRF immunity), but means any
  future XSS bug becomes full account takeover. No XSS found in this audit — React's
  default escaping is used consistently, no `dangerouslySetInnerHTML` anywhere — just
  flagging the blast radius so it's weighed if templating ever changes (e.g. rendering
  book descriptions as raw HTML). Accepted trade-off, not an actionable fix right now.

## Next — before opening to real community members

- ~~**Announcement board (admin)**~~ — `models.Announcement` + `AnnouncementHandler`
  (`internal/handlers/announcements.go`) give admins create/edit/toggle-active/delete CRUD at
  `/admin/announcements` (`src/app/admin/announcements/page.tsx`), still supporting multiple
  rows for scheduling/history. Community members only ever see the single newest active one
  in the `NotificationPanel` dropdown's "Announcement" section (`useActiveAnnouncements`
  picks `items[0]` off the `created_at desc`-ordered list) — deliberately not a stacked list;
  an admin wanting to say two things at once folds them into one announcement's body. See the
  "notification redesign" bullet under Later.
- **Feedback mechanism** — in-app "send feedback" form, writes to a table, emails the admin
  via the existing `EmailService`. No new infra.
- **Onboarding checklist** — surfaces the existing verification-status factors in the
  frontend; mostly UI, no new backend.
- **"Looking for" board** — the waitlist only works for a copy that already exists in the
  system; this covers "does anyone have X" for a book nobody's added yet.
- **Bulk/CSV import** and **ISBN scan-to-add** — reduces the friction of listing an entire
  shelf by hand; metadata lookup (Open Library/Google Books) already exists per-book.
- **Loan history / "my shelf" view** — currently-held + past loans per user.
- **"Report a problem" on a loan** — feeds into the existing `User.Suspended` field, which
  currently has no UI trigger.
- **"Automatic backup"** - backup settings, database (something like jellyfin?)
- **"Import settings"** - use yaml file to update settings
- **"Export and import books"** - allow users to export and import the books they have

## Later — once there's real usage to react to

- ~~**Surface announcements through the redesigned notification system**~~ — the
  Instagram/Facebook-style `NotificationPanel` dropdown now has a labeled "Announcement"
  section above the regular notification list (`useActiveAnnouncements` hook, fetched once
  per mount — not polled — surfacing only the single newest active announcement, dismissed
  client-side via `localStorage`, same as the deferred banner design; the bell/tab-bar badge
  count is unread notifications + 0-or-1 for the announcement). **A version changelog notice
  was intentionally left out of this pass** — see the new bullet below.
- **Version changelog notice** — a new app version becomes a notification-like entry in the
  same `NotificationPanel`; clicking it reveals the changelog, which should also be reachable
  from the existing `v{NEXT_PUBLIC_VERSION}` string already rendered in the footer
  (`src/app/layout.tsx`). Needs its own scoping pass: where changelog content comes from
  (hand-written per release? derived from commit/PR history?) and how "new version since the
  user's last visit" gets detected (compare `NEXT_PUBLIC_VERSION` against a value cached in
  `localStorage`, mirroring the announcement-dismissal pattern, is the leading idea).
- **Telegram bot: link account + receive notifications** — cheap relative to a from-scratch
  integration since `libs/telegram-bot-shared` and the bot generator already exist. Link via
  a deep-link token, forward `Notification` rows to Telegram alongside/instead of email.
- **Overdue reminders** — `LoanRequest.ExpectedReturnDate` and `Copy.ReturnDateRequired`
  already exist but nothing reads them proactively; add a scheduled job (alongside the
  existing cover-refresh job in `internal/services/scheduler.go`) that notifies
  borrower/owner via the existing `Notification` + `EmailService`. Highest-value cheap add —
  "silently overdue forever" is the most obvious failure mode once real people use this.
- A backup story for `data/bookshelf.db` + `data/covers` — needed now the URL is public.

## Someday / hold — don't build until there's proven demand

- **SSO** — only makes sense after the multi-tenant rewrite above; a single-community app
  with email/password + OTP doesn't need it.
- Book ratings/reviews, digest email, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter.
