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

## Shipped (recently — noted so it isn't re-proposed)

- **Author view** — `docs/author-view-spec.md`, `src/components/AuthorLink.tsx`,
  `src/app/authors/[author]`. Note: the spec file's own header still says "Draft, not yet
  approved for build" — stale, since the feature is live; update that header if you're next in
  the file.
- **"Import settings" via YAML** — `internal/handlers/appconfig.go`'s `LoadYAMLConfig`/
  `bookshelf.yaml`, unknown-key warning included.
- **Telegram bot: link account + receive notifications** — `apps/bookshelf-bot` (thin/stateless,
  `/start <token>` deep-link confirm) plus `internal/services/telegram.go` on the backend, per
  `docs/telegram-bot-integration-spec.md`.
- **Member "highly recommend this" thumbs-up** — `docs/book-recommendations-spec.md`,
  `internal/handlers/recommendations.go`.
- **Lending/community analytics** — per-book completed-loan counts, waitlist counts, catalog
  "Sort by: Popular" (`docs/community-reading-activity-spec.md`, issue #75). A genuinely open gap
  in the same space: that spec is per-book only, not an aggregate community-wide trend view
  (loan volume over time, top borrowers) — scope that as a new admin-dashboard addition if
  wanted, not a re-run of the shipped spec.
- **Multi-provider metadata enrichment (first slice)** — Hardcover added as a 4th identification
  provider alongside Open Library, Google Books, BookBrainz (`docs/metadata-search.md`'s
  "Hardcover specifics"). Further providers follow the same `fetchSource`/`sourcePriority`
  pattern if a real identification gap shows up — not planned speculatively. Goodreads
  considered and rejected (API key issuance long closed; scraping would violate its ToS).

## Next — before opening to real community members

- **Feedback mechanism** — in-app "send feedback" form, writes to a table, emails the admin
  via the existing `EmailService`. No new infra.

## Later — once there's real usage to react to

- **Guided tour for first-time users** — idea from comparing similar self-hosted apps; a
  first-run walkthrough for new members. Not yet spec'd. Revisit once there's a steadier trickle
  of brand-new (not invited-and-briefed) members — the app is currently invite-driven, so
  onboarding friction hasn't been confirmed as a real problem yet.
- **Overdue reminders** — `LoanRequest.ExpectedReturnDate` and `Copy.ReturnDateRequired`
  already exist, and `ExpectedReturnDate` is user-editable post-acceptance, but nothing reads it
  proactively yet. Two related asks point at the same fix: add a scheduled job (alongside the
  existing cover-refresh job in `internal/services/scheduler.go`) that (a) flags a loan once it's
  actually overdue and (b) nudges the borrower a day or two _before_ the due date — notifying via
  the existing `Notification` + `EmailService` (and now Telegram, since the bot link shipped).
  Highest-value cheap add — "silently overdue forever" is the most obvious failure mode once real
  people use this, and a before-the-fact reminder likely does more for a peer-trust lending model
  than an after-the-fact badge.
- **Authenticated home dashboard** — logged-in visits to `/` currently render the same
  `LandingPage` a logged-out visitor sees, just with the CTA swapped. A member with pending
  requests, a loan due back soon, or a waitlisted copy that opened up gets no summary — they have
  to separately check Loans, My Books, and the notification bell. Building blocks already exist
  (`CurrentlyBorrowedCard`, the pending-request counts in `my-books/page.tsx`,
  `useUnreadNotifications`) — this is assembly, not new data plumbing. Surfaced by a 2026-09-08
  internal UX review. Not started.
- **Bulk accept/decline for loan requests** — My Books already has bulk pause/delete for copies
  (`src/app/my-books/page.tsx`), but `my-books/[copyId]/requests` is still one-at-a-time,
  inconsistent with that pattern. Not started.
- **Staging/review workflow before a copy is added** — `copies_import.go`'s bulk import adds
  copies to the catalog instantly. A review step (stage → confirm/edit → finalize) would help
  when importing a large batch (e.g. a household's whole shelf) where some matches need
  correcting before going live. Not started.
- **OIDC/SSO login** — an auth-layer _addition_ alongside the existing homegrown email/password +
  OTP flow (`internal/handlers/auth.go`), not a replacement — communities self-hosting this app
  may already run an identity provider (Authentik, Authelia, etc.) they'd rather delegate to.
  Not started, no design work done. **Flagging a conflict**: this item used to live in
  "Someday/hold" below with the rationale "only makes sense after a multi-tenant rewrite" — that
  rationale assumed SSO implies per-community data isolation, but delegating auth to an existing
  IdP doesn't actually require multi-tenancy for a single-database app. Moved up here on that
  basis; revert if the multi-tenant concern still applies for a reason not captured above.

## Someday / hold — don't build until there's proven demand

- Book ratings/reviews, genre/tag browsing — nice-to-haves, revisit once
  there's enough real activity to know if they matter. Long-form reviews/ratings stay
  structurally out of scope per `apps/bookshelf-backend/CLAUDE.md`'s "Product scope" (link out to
  Google Books instead) — this entry is about lighter genre/tag browsing, which that guardrail
  doesn't rule out the same way.

## Considered and rejected

- **Waitlist position indicator** ("you're #2 in line") — `WaitlistButton.tsx` and
  `getWaitlistStatus` only track a total `count` and an `on_waitlist` boolean; there's no queue
  ordering in the data model to derive a position from. Would need waitlist entries to gain an
  ordering (e.g. join-timestamp-based rank) first — not a frontend-only change.

## Out of scope

Multi-format ebook/audiobook readers, Kobo/KOReader progress sync, OPDS catalog delivery, and
Send-to-Kindle-style features all require storing and serving actual book _files_ — a
content-storage subsystem this app doesn't have (`Book.CoverURL` is the only file-like field it
stores, a cached cover image, see `internal/services/covers.go`) and building one would
contradict the physical-lending product scope, not just extend it. Not planned.
