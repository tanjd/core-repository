# Bookshelf — TODO and ideas

Covers both `apps/bookshelf` (frontend) and `apps/bookshelf-backend` (API) as one product.
See `apps/bookshelf-backend/docker-compose.example.yml` for the current internal-testing
deployment (NAS, behind Tailscale).

## Done

- **Book watchlist / notifications** — join a waitlist for a loaned copy; borrowers are
  notified (`waitlist_available`) when it's returned. (`internal/handlers/waitlist.go`,
  `internal/services/loan_workflow.go`)
- **In-app notifications + email** — notification bell, Resend-backed transactional email
  with a dev-override address for testing.
- **Admin verification/trust factors** — email verified, phone on file, min-books-shared,
  surfaced via `/auth/verification-status`.
- **Website icon** — replaced the default Next.js favicon with an open-book mark (matching the
  `BookOpen` glyph used in `NavBar`) via `src/app/icon.svg` (App Router convention, modern
  browsers), a regenerated `favicon.ico` (16/32/48, legacy fallback), and `apple-icon.png` (180,
  iOS home-screen/touch icon).

## Now — internal testing (NAS, Tailscale-only)

- Deployed via `apps/bookshelf-backend/docker-compose.example.yml`; only the frontend port
  is published, backend stays on the internal compose network.
- **Sign-up gating for beta** — decided: admin approval (new accounts stay inactive until an
  admin approves them in the admin panel), not an invite code or fully open registration.
  Not yet implemented — needs a pending/approved state on `User` plus an admin UI action.

## Next — before opening to real community members

- **Announcement board (admin)** — admin-authored banner/list so beta users see "new
  feature," "known issue," etc. without a separate broadcast channel. New model + admin CRUD
  - a spot on the frontend layout, similar shape to the existing `AppSetting`/admin config.
- **Feedback mechanism** — in-app "send feedback" form, writes to a table, emails the admin
  via the existing `EmailService`. No new infra.
- **Onboarding checklist** — surfaces the existing verification-status factors in the
  frontend; mostly UI, no new backend.
- **Admin dashboard** — most-borrowed books, active lenders, overdue count, signups/week.
  All derivable from existing tables.
- **"Looking for" board** — the waitlist only works for a copy that already exists in the
  system; this covers "does anyone have X" for a book nobody's added yet.
- **Bulk/CSV import** and **ISBN scan-to-add** — reduces the friction of listing an entire
  shelf by hand; metadata lookup (Open Library/Google Books) already exists per-book.
- **Loan history / "my shelf" view** — currently-held + past loans per user.
- **Buy Me a Coffee** — a button and a link. Zero risk, do whenever.
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
- Rate limiting on `/register` and `/auth/otp/send`, and a backup story for
  `data/bookshelf.db` + `data/covers`, needed before the URL is public — not needed for
  Tailscale-only internal testing.

## Someday / hold — don't build until there's proven demand

- **Create communities (multi-tenant)** — `User` and every other model currently assume one
  org (the `User` model's own comment says "member of the church community"). This is a real
  architectural rewrite, not an incremental feature. Revisit only if a second group
  explicitly asks to run their own instance.
- **SSO** — only makes sense after the multi-tenant rewrite above; a single-community app
  with email/password + OTP doesn't need it.
- **Open-source plugin architecture** — large scope, premature before there's one proven
  successful deployment.
- Book ratings/reviews, digest email, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter.
