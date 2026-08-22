# How book metadata search works

Reference doc, not a spec — describes the current, implemented behavior of `GET
/books/metadata/search` end to end, so the pipeline can be explained (or reused elsewhere)
without re-deriving it from the three specs that built it piece by piece:
[`cross-edition-metadata-enrichment.md`](./cross-edition-metadata-enrichment.md) and
[`catalog-description-reconciliation-job.md`](./catalog-description-reconciliation-job.md). Update
this doc when the pipeline's shape changes; keep the specs as historical record of _why_ each
piece exists.

## Pipeline overview

```
GET /books/metadata/search?q=...
        │
        ▼
  searchMetadata (internal/handlers/metadata.go)
        │
        ├─ cache lookup (1h TTL, key = lowercased q [+ "|gbooks"]) — hit? return immediately.
        │
        ▼
  fetchAllSources(q)  ── fans out q, unchanged, to all 3 providers concurrently
        │
        ├─ Open Library   (fetchOpenLibrary)   — always
        ├─ Google Books   (fetchGoogleBooks)   — only if an API key is configured
        └─ BookBrainz     (fetchBookBrainz)    — always
        │
        ▼
  is q ISBN-shaped? ── expandSiblingEditions: resolve Title/Author from the ISBN hit(s),
        │              then fetchAllSources again with that Title+Author as q, and append
        │              those results too. (See "ISBN queries" below — this is the fix for
        │              editions with a different ISBN never surfacing.)
        │
        ▼
  consolidateResults (internal/handlers/metadata_consolidate.go)
        │
        ├─ 1. deduplicateIntoGroups — group same-book hits from different sources/queries
        ├─ 2. mergeGroup            — merge each group into one BookMetadataResult
        ├─ 3. enrichAcrossEditions  — backfill empty Description across sibling editions
        ├─ 4. sort                  — by completeness score, then Title
        │
        ▼
  cache.Set(cacheKey, consolidated) → response
```

`fetchAllSources`/`fetchOpenLibrary`/`fetchGoogleBooks`/`fetchBookBrainz` and
`expandSiblingEditions` live in `metadata.go` (the fetch layer — talks to the network).
`deduplicateIntoGroups`/`mergeGroup`/`enrichAcrossEditions`/`consolidateResults` and their helpers
live in `metadata_consolidate.go` (the pure layer — takes and returns `[]BookMetadataResult`, no
I/O, which is why it's the layer with actual unit test coverage; the fetch layer isn't
network-mocked in tests today).

## Step 1: grouping (`deduplicateIntoGroups`)

Keys primarily on normalized ISBN-13 (`normalizeISBN` upconverts ISBN-10 → ISBN-13 and strips
formatting). Falls back to normalized `title|author` (`normalizeTitleAuthor` — lowercase, strip
non-alphanumeric, collapse whitespace) **only** when a result has no ISBN of its own — e.g.
BookBrainz never returns one. A result carrying its own ISBN may still join an existing
title+author group (so a BookBrainz hit for an edition also seen elsewhere with an ISBN still
merges), but never once that group has already acquired a _different_ confirmed ISBN — otherwise
two genuinely distinct editions would silently collapse into one. See `findExistingGroup`'s
doc comment for the exact rule.

This is deliberately exact-match only — no fuzzy/similarity matching, no such library in `go.mod`.
An omnibus/anthology edition can share a normalized key with a single-volume edition; this is an
accepted, documented risk (see `TestCreateBook_DoesNotDedupByISBN_WhenOLKeyProvided` in
`books_test.go`), not something this pipeline tries to resolve.

## Step 2: merging (`mergeGroup`)

Within a group (same book, multiple source hits), each field takes the first non-empty/non-zero
value in source-priority order: `google_books` > `openlibrary` > everything else (`bookbrainz`),
via `sourcePriority`. `firstNonEmpty`/`firstNonZero` are the generic "first populated value in
priority order" helpers used for every field.

## Step 3: cross-edition backfill (`enrichAcrossEditions`)

Operates on the **already-merged, one-row-per-edition** list, bucketed by the same
`normalizeTitleAuthor` key grouping used in step 1 — reused rather than a second confidence
measure. A bucket with ≥2 editions means genuinely different ISBNs for the same work (if they
shared an ISBN they'd already be one row from step 1).

Only `Description` is cross-filled, backfill-only (never overwrites a populated field), and
skipped when both sides have a set, differing `Language`. `CoverURL`, `Publisher`,
`PublishedDate`, `PageCount`, and every identity field (`ISBN`, `OLKey`, `GoogleBooksID`,
`BookBrainzID`, `Title`, `Author`) are never touched — `CoverURL` has a frontend fallback already
and a wrong cover is a sharper problem for peer-to-peer lending than a wrong description; the
others are genuinely edition-specific facts. See `cross-edition-metadata-enrichment.md` for the
full reasoning. Every backfilled result is stamped in `EnrichedFields` (currently only ever
`["description"]`) so the frontend can label it — never a silent, unqualified claim.

`WorkKey` (same bucket key) is stamped onto every result with a non-empty Title+Author, for the
frontend to cluster distinct editions into one collapsed/expanded card
(`apps/bookshelf/src/app/share/components/MetadataSearchStep.tsx`) — a purely additive,
display-only concern; `consolidateResults` still returns one flat, score-sorted list.

## ISBN queries: why they used to return only one bare edition

`fetchAllSources` sends `q` to each provider **unchanged**. When `q` is an ISBN, each provider's
search only matches results indexed under that _exact_ ISBN — a sibling edition (different
printing, hardcover vs. paperback, different territory) has a different ISBN and simply never
appears in the fetched result set. Steps 1–3 above only ever operate on what got fetched, so no
amount of grouping/merging/backfilling logic could help: the sibling editions were never in the
pool to begin with. This is why searching ISBN `9781433532337` (a bare-info edition of "Church
Discipline") used to return one sparse row, even though searching "church discipline" directly
surfaced two editions, one with a cover.

**Fix**: `expandSiblingEditions`, called from `searchMetadata` right after the first
`fetchAllSources` when `normalizeISBN(q) != ""`. It picks the best available Title/Author from the
ISBN hit(s) (`bestTitleAuthorForExpansion` — same source-priority/completeness ordering as
`mergeGroup`/backfill donor selection) and re-runs `fetchAllSources` with `"<title> <author>"` as
the query, appending those results to the pool before `consolidateResults` runs. No new grouping
logic was needed — this widens what gets fetched, and lets the existing pipeline do the rest. If
the ISBN hit carried no usable Title/Author (all three providers failed, or returned bare
ISBN-only stubs), the expansion is skipped and the response is exactly what it was before this
fix — no regression on the failure path.

This roughly doubles the external calls made per ISBN query (two rounds of 2–3 providers instead
of one). Both rounds share the same `cache.Set` at the end, so a repeated identical query is still
one cache hit, not two more fan-outs.

## Known limitations

- **Bucket/backfill matching is exact-normalized-string only.** If the winning source for one
  edition formats the author differently than another edition's only source does (e.g. "William
  Kennedy" vs. "Kennedy" — a real provider inconsistency), their bucket keys diverge and neither
  grouping nor backfill fires, even though a human would recognize them as the same work. See
  `TestConsolidateResults_KnownLimitation_AuthorFormatDivergenceFromMergeBlocksBackfill`.
- **Persisted `Book` rows are a separate problem.** Everything above is scoped to the ephemeral
  `/books/metadata/search` response. Already-persisted catalog rows get their own reconciliation
  path — see `catalog-description-reconciliation-job.md` — and merging two `Book` rows into one
  (repointing `Copy`/`WishlistRequest.FulfilledBookID`, etc.) is still unaddressed anywhere.
- **ISBN expansion can't recover from a completely empty first round.** If all three providers
  fail or return no usable Title/Author for the ISBN itself, there's nothing to expand from.

## Where to look for each concern

| Concern                                                                             | File                                                             |
| ----------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Route registration, request/response shape, provider fan-out, ISBN expansion        | `internal/handlers/metadata.go`                                  |
| Grouping, merging, cross-edition backfill, scoring, ISBN/title-author normalization | `internal/handlers/metadata_consolidate.go`                      |
| Search result cache (in-memory, 1h TTL)                                             | `internal/handlers/metadata_cache.go`                            |
| Frontend bucketed-card display, "from another edition" label                        | `apps/bookshelf/src/app/share/components/MetadataSearchStep.tsx` |
| Scheduled reconciliation for already-persisted `Book` rows                          | `catalog-description-reconciliation-job.md`                      |
