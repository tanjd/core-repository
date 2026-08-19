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
- **Bulk/CSV import** and **ISBN scan-to-add** — reduces the friction of listing an entire
  shelf by hand; metadata lookup (Open Library/Google Books) already exists per-book.
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
