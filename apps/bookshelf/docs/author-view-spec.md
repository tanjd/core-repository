# Author View — spec

**Status:** Draft, not yet approved for build · **Scope:** `apps/bookshelf` +
`apps/bookshelf-backend` · **Depends on:** `Book`

Let a member tap an author's name from anywhere it appears and land on a page listing every book
by that author our community owns — inspired by author views in similar self-hosted apps, scoped
down to fit this product's exchange-focused mandate rather than copied wholesale.

## Why this shape, not a full author profile

Author views in similar apps lean toward book-discovery (bio, external bibliography, cover wall).
Both `apps/bookshelf/CLAUDE.md` and `apps/bookshelf-backend/CLAUDE.md` explicitly rule that out
for this product — no richer metadata pages, no "more like this," link out to Google Books
instead. This spec keeps the feature to _"what does our community already own by this author"_ —
a filtered slice of the existing catalog, not a new discovery surface. That reframing is what
keeps this in scope without needing a guardrail amendment (contrast with
`book-recommendations-spec.md`, which needed one).

## Goals

- Any author name shown in the app (catalog card, catalog row, book detail page) is tappable and
  leads to a page listing every book by that author currently in the catalog.
- That page reuses the exact same book-card/list presentation as the catalog — same information
  density, same "available"/"loaned" status treatment — just pre-filtered to one author.
- Works identically on mobile and desktop, without claiming a nav-bar or tab-bar slot: it's
  reached by tapping an author name, not through primary navigation.

## Non-goals (v1)

- **Author bio, photo, external bibliography, "other works" beyond our catalog.** All discovery
  metadata — explicitly out of scope per the product-scope guardrail. Link out to Google Books
  if a member wants that.
- **Author name normalization/canonicalization.** Author is free-text today (no separate entity).
  "Frank Herbert" and "Herbert, Frank" are different authors to this feature. Fixing that is a
  separate, bigger effort (data model change) and not a precondition for shipping v1 — an author
  page just won't catch every variant spelling until that's addressed.
- **Own top-level nav entry or tab-bar slot.** Reached only by tapping an author name.
- **Sorting/filtering within the author page beyond what the catalog already offers** (e.g. no
  bespoke "sort by publish year" — we don't carry that field).

## Behaviors

- **Entry points.** Author name becomes a tappable link everywhere it's shown as identifying
  metadata on a book (catalog card, catalog row, detail page). Author text used purely as
  fallback/loading typography (e.g. a cover placeholder) is not a link — nothing to navigate to
  yet in that context.
- **Matching.** The page shows books whose author field exactly matches the tapped author string.
  No fuzzy/partial matching in v1 (see non-goals).
- **Empty case.** Can't actually occur in-app (the link only ever comes from a book that has that
  author), so no empty-state design is needed for v1.
- **Pagination.** If an author has enough books to need it, the page paginates the same way the
  catalog does — no special-cased "show all" behavior.
- **Status/availability.** Each book on the author page shows the same available/loaned badge
  treatment as everywhere else in the app (the shared four-variant badge vocabulary) — no
  author-page-specific styling.

## Scope-guardrail note

Unlike `book-recommendations-spec.md`, this feature does **not** require an amendment to the
product-scope guardrail in `apps/bookshelf/CLAUDE.md` / `apps/bookshelf-backend/CLAUDE.md` — it
surfaces existing catalog data grouped by an existing field, adding no new metadata and no
external-source content. If a future revision of this feature wants to add bio/external
bibliography, that _would_ need the same guardrail conversation `book-recommendations-spec.md`
went through.

## Open questions to resolve at approval time

- **Exact-match limitation:** acceptable for v1, or does inconsistent author-string formatting in
  the current catalog make this feel broken enough to need a normalization pass first? (Depends
  on how messy today's data actually is — worth a quick look at distinct `author` values before
  approving.)
- **Route/endpoint shape:** left to the implementer per this spec's own convention (intent, not
  routes) — but flagging that the natural shapes are a new backend list-endpoint filtered by
  exact author, and a new frontend route keyed by an encoded author string.

## How we'll know it's working

- Author links get tapped in the wild (worth instrumenting, or just checking whether members
  mention using it) — if nobody uses it, don't invest in fixing the exact-match limitation.
