# Bookshelf — TODO and ideas

Covers both `apps/bookshelf` (frontend) and `apps/bookshelf-backend` (API) as one product.
The app is now live publicly (see `compose/docker-compose.bookshelf.yml` — Traefik +
Cloudflare, domain `bookshelf.tanjd.com`), superseding the old Tailscale-only internal
testing setup `apps/bookshelf-backend/docker-compose.example.yml` still documents.

## Security — from the 2026-08-18 audit

Full audit covered every route's auth/ownership check, JWT/crypto/email code, and the
production compose file. Two findings were fixed immediately (low-risk, self-contained);
the rest need a deliberate follow-up since they touch infra or have UX trade-offs
(rate-limit thresholds, etc.).

**Fixed:**

- ~~HTML/email injection via unescaped `User.Name`/`Book.Title` in transactional emails~~ —
  `internal/services/loan_workflow.go` and `internal/handlers/auth.go` now run every
  user-controlled field through `html.EscapeString` before interpolating into the
  `text/html` email bodies. Was exploitable via a crafted display name or book title
  rendering as a live link/markup in emails sent to _other_ users (phishing vector).
- ~~Directory listing on `/covers/`~~ — `cmd/server/main.go`'s cover static handler now
  404s any request path ending in `/` instead of falling through to
  `http.FileServer`'s default listing.

**Still open, ranked by severity:**

1. **No rate limiting on `/auth/login`, `/auth/register`, `/auth/send-otp`.** No throttle
   at the app or Traefik layer. bcrypt slows brute force but there's no lockout — open to
   credential stuffing and OTP-inbox-flooding now that the URL is public, not just on
   Tailscale. (This supersedes the old "Later" bullet below — now urgent.) Add per-IP/
   per-account rate limiting, e.g. `chi`'s `httprate` in the Go app, or a Traefik
   `rateLimit` middleware label on the compose file.
2. **Backend (8000) and frontend (3000) ports are both published directly to the host**
   in `compose/docker-compose.bookshelf.yml`, on top of the Traefik `proxy` network
   wiring. If the NAS/router forwards or LAN-exposes those ports, requests bypass
   Traefik's TLS termination — the Go API would serve plaintext HTTP with JWTs in the
   `Authorization` header. Drop the `ports:` blocks for both services; only the `proxy`/
   `bookshelf` networks should need to reach them.
3. **Possible SMTP header injection via the new-email field.** `requestEmailChange`
   (`internal/handlers/auth.go`) passes client-supplied `newEmail` straight into the `To:`
   header of a raw MIME message built with `fmt.Sprintf` (`internal/services/email.go`).
   Go's `net/smtp` blocks CRLF in the SMTP envelope command but not in this header-string
   construction. Explicitly reject any value containing `\r`/`\n` before it's used in a
   header line, regardless of huma's `format:"email"` validation.
4. **`/auth/setup` has a TOCTOU race** — `HasAdmin()` is checked, then an admin is
   created, with no transaction tying the two together
   (`internal/handlers/auth.go:556-563`). Two concurrent requests in the window between
   deploy and the real admin's first setup call could both succeed. Narrow window, cheap
   fix (wrap in a transaction / unique constraint check).
5. **No CSP/HSTS/X-Frame-Options/X-Content-Type-Options anywhere** — not in
   `next.config.ts`, not in the Go backend, not in the Traefik labels. Likely relying on
   Cloudflare's proxy defaults for HSTS, but there's no CSP at all, and the app renders
   book descriptions from three external metadata sources (Open Library, Google Books,
   BookBrainz).
6. **JWT stored in `localStorage`** (`src/lib/api.ts`) rather than an httpOnly cookie.
   Standard trade-off for a Bearer-token SPA (and it buys CSRF immunity), but means any
   future XSS bug becomes full account takeover. No XSS found in this audit — React's
   default escaping is used consistently, no `dangerouslySetInnerHTML` anywhere — just
   flagging the blast radius so it's weighed if templating ever changes (e.g. rendering
   book descriptions as raw HTML).

## Next — before opening to real community members

- **Announcement board (admin)** — admin-authored banner/list so beta users see "new
  feature," "known issue," etc. without a separate broadcast channel. New model + admin CRUD
  - a spot on the frontend layout, similar shape to the existing `AppSetting`/admin config.
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

## Later — once there's real usage to react to

- **Telegram bot: link account + receive notifications** — cheap relative to a from-scratch
  integration since `libs/telegram-bot-shared` and the bot generator already exist. Link via
  a deep-link token, forward `Notification` rows to Telegram alongside/instead of email.
- **Overdue reminders** — `LoanRequest.ExpectedReturnDate` and `Copy.ReturnDateRequired`
  already exist but nothing reads them proactively; add a scheduled job (alongside the
  existing cover-refresh job in `internal/services/scheduler.go`) that notifies
  borrower/owner via the existing `Notification` + `EmailService`. Highest-value cheap add —
  "silently overdue forever" is the most obvious failure mode once real people use this.
- A backup story for `data/bookshelf.db` + `data/covers` — needed now the URL is public.
  (Rate limiting on `/register`/`/auth/send-otp`, previously noted here, moved to the
  "Security" section above as urgent — it's now open item #1 there.)

## Someday / hold — don't build until there's proven demand

- **Create communities (multi-tenant)** — `User` and every other model currently assume one
  org (the `User` model's own comment says "member of the church community"). This is a real
  architectural rewrite, not an incremental feature. Revisit only if a second group
  explicitly asks to run their own instance.
- **SSO** — only makes sense after the multi-tenant rewrite above; a single-community app
  with email/password + OTP doesn't need it.
- Book ratings/reviews, digest email, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter.
