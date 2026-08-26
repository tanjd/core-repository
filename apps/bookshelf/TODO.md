# Bookshelf — TODO and ideas

Covers both `apps/bookshelf` (frontend) and `apps/bookshelf-backend` (API) as one product.
The app is now live publicly (see `compose/docker-compose.bookshelf.yml` — Traefik +
Cloudflare, domain `bookshelf.tanjd.com`), superseding the old Tailscale-only internal
testing setup `apps/bookshelf-backend/docker-compose.example.yml` still documents.

**Still open:**

- **JWT stored in `localStorage`** (`src/lib/api.ts`) rather than an httpOnly cookie.
  Standard trade-off for a Bearer-token SPA (and it buys CSRF immunity), but means any
  future XSS bug becomes full account takeover. No XSS found in this audit — React's
  default escaping is used consistently, no `dangerouslySetInnerHTML` anywhere — just
  flagging the blast radius so it's weighed if templating ever changes (e.g. rendering
  book descriptions as raw HTML). Accepted trade-off, not an actionable fix right now.

## Next — before opening to real community members

- **Feedback mechanism** — in-app "send feedback" form, writes to a table, emails the admin
  via the existing `EmailService`. No new infra.
- **"Import settings"** - use yaml file to update settings

## Later — once there's real usage to react to

- **Telegram bot: link account + receive notifications** — cheap relative to a from-scratch
  integration since `libs/telegram-bot-shared` and the bot generator already exist. Link via
  a deep-link token, forward `Notification` rows to Telegram alongside/instead of email.
- **Overdue reminders** — `LoanRequest.ExpectedReturnDate` and `Copy.ReturnDateRequired`
  already exist, and `ExpectedReturnDate` is now user-editable post-acceptance (see "Shipped"
  above), but nothing reads it proactively yet; add a scheduled job (alongside the existing
  cover-refresh job in `internal/services/scheduler.go`) that notifies borrower/owner via the
  existing `Notification` + `EmailService`. Highest-value cheap add — "silently overdue
  forever" is the most obvious failure mode once real people use this.

## Someday / hold — don't build until there's proven demand

- **SSO** — only makes sense after the multi-tenant rewrite above; a single-community app
  with email/password + OTP doesn't need it.
- Book ratings/reviews, digest email, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter.
