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
        ├─ 4. sort                  — by completeness score, then Title (case-insensitive)
        │
        ▼
  is q ISBN-shaped? ── promoteQueriedEdition(consolidated, normalizeISBN(q)): pin the result
        │              whose own ISBN matches q to #1, regardless of completeness score. (See
        │              "Ranking the exact-ISBN edition first" below.) Called from searchMetadata
        │              itself, after consolidateResults returns — not part of consolidateResults.
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

## Google Books needs the `isbn:` operator for an ISBN query (`googleBooksQueryFor`)

`fetchAllSources` sends `q` unchanged to every provider (see above), but Google Books' free-text
search does not reliably match a bare ISBN string — verified live: `?q=9781433532337` returns
`totalItems: 0` for a real, indexed ISBN, while `?q=isbn:9781433532337` returns exactly the right
edition. Without this, the first round of an ISBN search got **no** Google Books contribution at
all, leaving only Open Library's sparse, work-level hit (see below) in the pool.

`googleBooksQueryFor` (`metadata.go`) rewrites the query to `"isbn:" + normalizeISBN(q)` whenever
`q` is ISBN-shaped, and passes non-ISBN queries through unchanged — so `expandSiblingEditions`'s
later title+author re-fetch is unaffected. This is scoped to Google Books only: Open Library
already matches a bare ISBN string correctly for this case (confirmed live), and BookBrainz's
search is unrelated/noisy for this kind of query regardless of prefix.

## Ranking the exact-ISBN edition first (`promoteQueriedEdition`)

`expandSiblingEditions` above exists so a sparse ISBN hit can be backfilled from a richer sibling —
but nothing about completeness-score ranking (step 4) guarantees the literal edition the user
searched for stays the top pick once siblings are in the pool. Real example: searching ISBN
`9781433532337` ("Church Discipline" by Jonathan Leeman), once Google Books actually returns it
(see `googleBooksQueryFor` above), gets the correct English edition — cover, description,
publisher, but `PageCount: 0` (Google's own catalog entry is incomplete on that one field) — while
`expandSiblingEditions`'s broader title+author query surfaces several translated siblings, one of
which (a Burmese edition) Google reports with a non-zero page count. That single extra point is
enough for the Burmese edition to genuinely outscore the correct one under `scoreResult` — not a
tie, a real score difference.

`promoteQueriedEdition` (`metadata_consolidate.go`) fixes this directly: called from
`searchMetadata` right after `consolidateResults`, it moves the result whose own (normalized) ISBN
equals the queried ISBN to index 0, unconditionally — completeness score no longer decides the #1
slot once the query itself is an unambiguous identifier. It's a no-op when the query wasn't
ISBN-shaped, or when no fetched result actually carries the queried ISBN (the existing "ISBN
expansion can't recover from a completely empty first round" limitation below still applies in that
case — there's nothing to promote).

**Important dependency**: `promoteQueriedEdition` can only promote a result that's actually in the
pool. Open Library folds every edition of a work into one search hit and reports only one
representative ISBN (`isbn[0]` of a possibly-long array, in whatever order OL's index happens to
store it) — for this book, that's `9781433532368`, a _sibling_ of the queried `9781433532337`, not
the queried ISBN itself. So Open Library alone was never enough for `promoteQueriedEdition` to find
a match; it depends on Google Books actually returning the literal queried ISBN, which is exactly
what `googleBooksQueryFor` fixes. The two fixes are a pair — either alone leaves this case broken.

This only covers ISBN queries. A free-text title/author query has no single "the thing that was
literally asked for" to pin to — see "Future direction" below.

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
- **Open Library's `language` field is unreliable and isn't used for ranking.** Many translated
  editions are mistagged `"eng"` in OL's own index (e.g. Burmese/Chinese/Kurdish translations of
  "Church Discipline" by Jonathan Leeman all report `language: ["eng"]`), so language can't be used
  to deprioritize a mismatched-language sibling edition in the final sort — only the exact-string
  `Language` equality check in `enrichAcrossEditions`'s description backfill relies on it, and only
  as a best-effort skip, not a ranking signal. `promoteQueriedEdition` (above) sidesteps this for
  ISBN queries by pinning on identity rather than language; free-text queries have no such anchor
  and remain exposed to this.
- **Free-text query ranking has no equivalent to `promoteQueriedEdition`.** A title/author search
  has no single unambiguous "the thing that was asked for" to pin to the way an ISBN does, so a
  translated or otherwise tangential sibling edition can still legitimately win the top slot on raw
  completeness score for a plain-text query. This is a genuinely harder relevance problem than the
  ISBN case — see "Future direction" below.

## Future direction: a relevance regression suite

`promoteQueriedEdition` fixes the one case that was reported and verified against live data, but
completeness-score ranking is inherently going to keep producing edge cases like it — this is a
search relevance problem, not a single bug, and it matters for user experience. The next step,
deliberately not built in the same session as this fix, is a **fixture-based relevance regression
suite**: capture real (sanitized) API responses for known-tricky queries as static JSON fixtures
checked into the repo, and replay them through `consolidateResults`/`promoteQueriedEdition` in CI to
assert the expected top result — catching future regressions the way today's Church Discipline
scenario was caught by hand.

This needs one prerequisite: `fetchOpenLibrary`/`fetchGoogleBooks`/`fetchBookBrainz`
(`metadata.go`) currently call the package-level `metadataClient` directly, so the fetch layer has
no test coverage at all today (see the note above). They'd need to accept an injectable HTTP client
(or an interface seam) before recorded fixtures could stand in for live network calls. The
`TestConsolidateResults_PromoteQueriedEdition_ExactISBNBeatsHigherScoringSibling` test added
alongside this fix is a natural first candidate to seed that suite with, once the seam exists.

## Where to look for each concern

| Concern                                                                             | File                                                             |
| ----------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Route registration, request/response shape, provider fan-out, ISBN expansion        | `internal/handlers/metadata.go`                                  |
| Grouping, merging, cross-edition backfill, scoring, ISBN/title-author normalization | `internal/handlers/metadata_consolidate.go`                      |
| Pinning the exact-ISBN edition to #1 (`promoteQueriedEdition`)                      | `internal/handlers/metadata_consolidate.go`                      |
| Search result cache (in-memory, 1h TTL)                                             | `internal/handlers/metadata_cache.go`                            |
| Frontend bucketed-card display, "from another edition" label                        | `apps/bookshelf/src/app/share/components/MetadataSearchStep.tsx` |
| Scheduled reconciliation for already-persisted `Book` rows                          | `catalog-description-reconciliation-job.md`                      |
