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

## Already shipped — surfaced by this comparison, worth noting so it isn't proposed again

**Lending/community analytics.** Reading-analytics dashboards in similar apps mapped, for
bookshelf, to per-book completed-loan counts, waitlist counts, and a catalog "Sort by: Popular" —
this shipped already as `apps/bookshelf/docs/community-reading-activity-spec.md` (issue #75), well
before this comparison. A genuinely open gap in the same space: that spec is per-book only, not an
aggregate community-wide trend view (e.g. loan volume over time, top borrowers) — if that's wanted
later, scope it as a new admin-dashboard addition, not a re-run of the shipped spec.
