# Author View — spec

**Status:** Draft, not yet approved for build · **Scope:** `apps/bookshelf` +
`apps/bookshelf-backend` · **Depends on:** `Book`

Let a member tap an author's name from anywhere it appears — or browse an index of every author in
the catalog — and land on a page listing every book by that author our community owns — inspired
by author views in similar self-hosted apps, scoped down to fit this product's exchange-focused
mandate rather than copied wholesale.

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
- A member can also browse _all_ authors in the catalog and pick one, rather than only reaching
  the per-author page by tapping a name on a book they've already found (see "Authors index page"
  below).

## Non-goals (v1)

- **Author bio, photo, external bibliography, "other works" beyond our catalog.** All discovery
  metadata — explicitly out of scope per the product-scope guardrail. Link out to Google Books
  if a member wants that.
- **Author name normalization/canonicalization beyond display-key grouping (see below).** Author
  is free-text today (no separate `Author` entity, no `AuthorID` FK). Promoting it to a real
  entity — with an admin merge/alias tool, mirroring the still-unaddressed "merge two `Book` rows"
  gap noted in `cross-edition-metadata-enrichment.md` — is a separate, bigger effort and not a
  precondition for shipping v1. A spot-check of the current catalog (40 books, 9 distinct authors)
  found no variant-spelling collisions, so this is deferred until real duplicates actually show up
  in the wild, not built speculatively.
- **Own top-level nav entry or tab-bar slot.** Reached by tapping an author name, or via the
  Authors toggle nested inside the existing Catalog page — neither claims a tab-bar slot or a new
  primary nav destination.
- **Sorting/filtering within the author page beyond what the catalog already offers** (e.g. no
  bespoke "sort by publish year" — we don't carry that field).
- **Sorting/filtering within the authors index beyond alphabetical.** No "sort by book count," no
  search-within-authors box — the alphabetical sticky-header list is the whole v1 surface (see
  below).

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

## Authors index page

A new browsable entry point, additive to the tap-through behavior above: a member can see every
distinct author in the catalog and pick one, without first having found one of their books.

- **Entry point**: a segmented toggle at the top of the existing Catalog page — "Books" /
  "Authors" — rather than a new top-level route or nav/tab-bar destination. Switching to
  "Authors" replaces the book grid/table with the authors list below; switching back returns to
  the normal catalog view. This keeps the feature inside the page members already use to browse,
  consistent with the "no nav-bar or tab-bar slot" constraint above.
- **List design**: alphabetical, sticky-letter-header list (contacts-app pattern) — one row per
  distinct author, showing the author string as-typed and a book count (e.g. "3 books"). Tapping a
  row navigates to that author's existing filtered book-list page (same page the tap-through
  entry points already lead to).
- **Grouping key — exact-string only in v1.** Authors are grouped by the literal `Author` field
  value, the same exact-match rule the per-author page already uses — no normalization, no
  case-folding, no "Last, First" collapsing. "Frank Herbert" and "Herbert, Frank" would list as
  two separate rows. This mirrors the non-goal above: normalization is deferred, not solved
  differently for the index than for the tap-through link.
  - **Why this matters more here than for tap-through**: a missed tap-through link just does
    nothing; a duplicate-looking row in a list a member is actively scanning reads as a bug. If
    real duplicate authors show up as the catalog grows, the cheapest next step — _before_
    reaching for a data-model change — is grouping by a normalized display key (lowercase, strip
    punctuation/whitespace, collapse "Last, First" → "First Last") computed at read time, reusing
    `normalizeTitleAuthor`'s same "exact-after-normalization, no fuzzy matching" bar from
    `internal/handlers/metadata_consolidate.go` rather than inventing a second confidence measure.
    This is a future option to reach for if needed, not part of this spec's v1 scope.
- **Pagination**: same treatment as the catalog and per-author pages — paginate if the author
  count grows large enough to need it, no special-cased "show all."
- **Empty case**: if the catalog itself is empty, the Authors toggle can still render (unlike the
  per-author page's empty case, this one _can_ occur) — show the same empty state the Books view
  already shows for an empty catalog, reworded for authors if needed.

## Scope-guardrail note

Unlike `book-recommendations-spec.md`, this feature does **not** require an amendment to the
product-scope guardrail in `apps/bookshelf/CLAUDE.md` / `apps/bookshelf-backend/CLAUDE.md` — it
surfaces existing catalog data grouped by an existing field, adding no new metadata and no
external-source content. If a future revision of this feature wants to add bio/external
bibliography, that _would_ need the same guardrail conversation `book-recommendations-spec.md`
went through.

## Open questions to resolve at approval time

- **Exact-match limitation:** checked against the current catalog (40 books, 9 distinct authors)
  — no variant-spelling collisions found today, so exact-match is acceptable for v1 on both the
  tap-through link and the new authors index. Revisit if the catalog grows and duplicates actually
  appear (see the display-key-grouping fallback option above).
- **Route/endpoint shape:** left to the implementer per this spec's own convention (intent, not
  routes) — but flagging that the natural shapes are a new backend list-endpoint filtered by
  exact author, a new backend endpoint (or a query param on the existing one) returning distinct
  authors + counts for the index, and a new frontend route keyed by an encoded author string.

## How we'll know it's working

- Author links get tapped in the wild (worth instrumenting, or just checking whether members
  mention using it) — if nobody uses it, don't invest in fixing the exact-match limitation.
