# Roadmap: feature ideas from other self-hosted apps

Not a spec — a scoped backlog. This doc records ideas surfaced while comparing bookshelf against
other self-hosted community/library-style apps, so the comparison doesn't need re-deriving each
time a similar project comes up. Bookshelf is a community _physical_ book-lending app (see
`apps/bookshelf-backend/CLAUDE.md`'s "Product scope") — its job is identifying which books the
community already owns and facilitating request/loan/return, not being a digital library or
reader, so most flagship surfaces of digital-library-style apps don't transplant here.

## Out of scope — noted so it isn't re-litigated

Multi-format ebook/audiobook readers, Kobo/KOReader progress sync, OPDS catalog delivery, and
Send-to-Kindle-style features all require storing and serving actual book _files_ — a
content-storage subsystem this app doesn't have today (`Book.CoverURL` is the only file-like field
it stores, and that's just a cached cover image, see `internal/services/covers.go`). Building one
would also directly contradict the documented physical-lending product scope, not just extend it.
Not planned.

## In scope

### Multi-provider metadata enrichment — first slice shipped

Adding metadata _providers_ to the `internal/handlers/metadata.go` search fan-out is in scope when
the purpose is closing identification gaps (see the Product-scope amendment this roadmap prompted,
in `apps/bookshelf-backend/CLAUDE.md`) — not for enrichment fields like ratings/reviews, which stay
out per that same amendment. Hardcover was added as the 4th provider (alongside Open Library,
Google Books, BookBrainz) — see `docs/metadata-search.md`'s "Hardcover specifics" section for the
implementation. Goodreads was considered and rejected: its public API key issuance has been closed
for years, and scraping it would violate its ToS.

Further providers can follow the same pattern (`fetchSource` entry in `fetchAllSources`, a
`sourcePriority` tier, a `fetch<Provider>` function) if a real identification gap shows up in
practice — not planned speculatively.

### OIDC/SSO login

Bookshelf's auth (`internal/handlers/auth.go`) is homegrown: email/password plus OTP-based
verification and registration. Adding an OIDC/SSO login option (a common feature in similar
self-hosted apps) would be an auth-layer addition alongside the existing flow, not a
replacement — communities self-hosting this app may already run an identity provider (Authentik,
Authelia, etc.) they'd rather delegate to. Not started; no design work done yet.

### Staging/review workflow before a copy is added

`copies_import.go`'s bulk import currently adds copies to the catalog instantly. A review step
(stage → confirm/edit → finalize) — a pattern seen in similar apps' upload/review/finalize flows
for new library content — would help when importing a large batch (e.g. a household's whole shelf)
where some matches need correcting before they're live. Not started.

## In scope — from internal UX review (2026-09-08)

Not from the other-apps comparison above — surfaced by reviewing the existing frontend flows
directly. Recorded here anyway since this is the established "in scope, not started" backlog.

### Authenticated home dashboard

`apps/bookshelf/src/app/page.tsx` routes every logged-in visit through the same
`LandingPage` a logged-out visitor sees (marketing hero/screenshots/feature grid), just with the
CTA swapped to "Go to catalog." A member with pending requests to review, a loan due back soon, or
a waitlisted copy that opened up gets no summary of any of that — they have to separately check
Loans, My Books, and the notification bell. The building blocks already exist
(`CurrentlyBorrowedCard`, the pending-request counts computed in `my-books/page.tsx`,
`useUnreadNotifications`) — this would be assembling them into a real "here's what needs your
attention" view for authenticated users, not new data plumbing. Not started.

### Proactive due-date reminders

The only due-date signal today is an "Overdue" badge that appears _after_ the return date passes
(`isOverdue` in `apps/bookshelf/src/lib/loanStatus.ts`) — nothing nudges a borrower beforehand. For
a peer-lending app where the trust model depends on timely returns, a reminder a day or two before
the due date (reusing the existing email/Telegram notification channels) would likely reduce
overdue rates more than an after-the-fact badge does. Needs a small backend piece — a scheduled job
querying loans by due date, alongside the existing digest/notification dispatch. Not started.

### Bulk accept/decline for loan requests

My Books already has bulk pause/delete for copies (`apps/bookshelf/src/app/my-books/page.tsx`), but
request management on `my-books/[copyId]/requests` is still one-at-a-time. An owner with several
books and a pile of pending requests has no way to accept/decline in bulk, which is inconsistent
with the bulk-action pattern already shipped for copies. Not started.

### Considered and rejected: waitlist position indicator

Showing "you're #2 in line" was considered but rejected — `WaitlistButton.tsx` and the
`getWaitlistStatus` API only track a total `count` and an `on_waitlist` boolean; there's no queue
ordering in the data model to derive a position from. Would need waitlist entries to gain an
ordering (e.g. a join-timestamp-based rank) before a position indicator is meaningful — not a
frontend-only change like the two above.

## Already shipped — surfaced by this comparison, worth noting so it isn't proposed again

**Lending/community analytics.** Reading-analytics dashboards in similar apps mapped, for
bookshelf, to per-book completed-loan counts, waitlist counts, and a catalog "Sort by: Popular" —
this shipped already as `apps/bookshelf/docs/community-reading-activity-spec.md` (issue #75), well
before this comparison. A genuinely open gap in the same space: that spec is per-book only, not an
aggregate community-wide trend view (e.g. loan volume over time, top borrowers) — if that's wanted
later, scope it as a new admin-dashboard addition, not a re-run of the shipped spec.
