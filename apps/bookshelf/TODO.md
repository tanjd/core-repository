# Bookshelf — TODO and ideas

Covers both `apps/bookshelf` (frontend) and `apps/bookshelf-backend` (API) as one product.
See `apps/bookshelf-backend/docker-compose.example.yml` for the current internal-testing
deployment (NAS, behind Tailscale).

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
- Book ratings/reviews, digest email, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter.
