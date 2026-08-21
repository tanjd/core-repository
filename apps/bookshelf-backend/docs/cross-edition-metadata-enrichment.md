# Spec: cross-edition metadata enrichment in search results

Status: **not yet implemented** — spec only, for a future dev session to pick up.

## Problem

`GET /books/metadata/search` fans out to Open Library, Google Books, and BookBrainz, then
`consolidateResults` (`internal/handlers/metadata_consolidate.go`) groups the results into
distinct books and ranks them. Grouping (`deduplicateIntoGroups`) keys primarily on normalized
ISBN-13, only falling back to normalized title+author when ISBN is _absent_ on a result.

When the same work genuinely has two different real ISBNs (hardcover vs. paperback, different
territory printing, etc.), each stays its own group — and one edition can come back with a cover

- description while another, equally valid edition of the _same book_ comes back bare, purely
  because whichever provider indexed that specific ISBN didn't have the field. This was observed
  directly: searching for a title returns two results under different ISBNs, one richer than the
  other, with no way to fill the gap from the sibling result.

## Scope

- **In scope**: enrich search results only — the ephemeral `[]BookMetadataResult` produced by
  one `/books/metadata/search` call, before caching/returning. No DB writes.
- **Out of scope (deferred)**: backfilling already-persisted `Book` catalog rows across editions.
  That's a separate, higher-stakes problem — it would touch real rows with attached `Copy`
  records (physical items, owners, loan history) and `WishlistRequest.FulfilledBookID`, and
  raises questions (true merge vs. field-only backfill, repointing copies, etc.) that were
  explicitly scoped out of this change during design discussion. Revisit separately if wanted.
- **Trigger**: automatic, above a high-confidence bar — no admin review step. Mirrors the trust
  level already given to `deduplicateIntoGroups`'s own title+author fallback path (same
  normalization function, same "no fuzzy matching, exact-after-normalization only" bar — there is
  no fuzzy/similarity library in `go.mod` today, and none is being introduced by this change).
- **Backfill only**: never overwrites a field that's already populated.

## Design

Add an `enrichAcrossEditions` pass in `consolidateResults`, after the per-group `mergeGroup` step
and before the final sort:

```go
groups := deduplicateIntoGroups(results)
merged := make([]BookMetadataResult, 0, len(groups))
for _, group := range groups {
    merged = append(merged, mergeGroup(group))
}
merged = enrichAcrossEditions(merged)   // new
sort.SliceStable(merged, ...)
```

`enrichAcrossEditions` buckets the already-merged, one-per-edition results by
`normalizeTitleAuthor(r.Title, r.Author)` — the same key `deduplicateIntoGroups` already uses;
reuse it rather than inventing a second confidence measure. Entries with empty `Title` or empty
`Author` are excluded from bucketing (an empty-key bucket would otherwise wrongly lump unrelated
sparse results together). Buckets of size 1 are no-ops.

Within a bucket (≥2 distinct ISBN editions of the same title+author), for each member with an
empty field, fill it from the best other member in the bucket, ordered by `sourcePriority` then
`scoreResult` (the same "richest/most trusted source first" ordering `mergeGroup` already uses),
computed against a stable snapshot of the bucket so fills don't cascade or depend on iteration
order.

### Field scope: only `CoverURL` and `Description`

These are work-level facts that hold across editions. `Publisher`, `PublishedDate`, and
`PageCount` are genuinely **edition-specific** — a different ISBN can legitimately have a
different page count or publisher — so those must **not** be cross-filled, or a sparse edition
would silently display factually wrong specifics for the ISBN the user is about to pick.
`Title`, `Author`, `ISBN`, `OLKey`, `GoogleBooksID`, `BookBrainzID` are never touched by this
pass — copying identity keys across editions would corrupt `findExistingBook`'s downstream dedupe
(`internal/handlers/books.go`) if the enriched result is later used to create a `Book`.

### Language guard on `Description`

Skip a candidate donor whose `Language` is non-empty and differs (case-insensitive) from the
target's non-empty `Language` — otherwise a French edition's description could get attached to an
English edition's search result. If either side's `Language` is unset, proceed (best-effort; most
results in practice are single-market/English).

### Transparency field

Add `EnrichedFields []string` (`json:"enriched_fields,omitempty"`) to `BookMetadataResult`
(`internal/handlers/metadata.go`, near line 38), listing which fields (`"cover_url"`,
`"description"`) were filled from a different edition. Additive, `omitempty`, doesn't break
existing struct-equality tests. Lets the frontend later show something like "cover from another
edition" if desired — not required for this change to be useful on its own, just cheap to add
now.

## Edge cases to cover (design + tests)

1. Two editions, same normalized title+author, different ISBN — sparse one gets `CoverURL` and
   `Description` filled from the richer one; the richer one is left untouched.
2. `Publisher`/`PublishedDate`/`PageCount` must stay empty/zero on the sparse result even though
   the donor has them — these are never cross-filled.
3. Never overwrite an already-non-empty field, even with a "better" value from a higher-priority
   source.
4. `ISBN`/`OLKey`/`GoogleBooksID`/`BookBrainzID`/`Title`/`Author` are never copied across
   editions.
5. Entries with empty `Title` or empty `Author` are excluded from bucketing.
6. Description fill is skipped when donor and target `Language` are both set and differ; proceeds
   when either is empty.
7. Multiple candidate donors in one bucket — selection is deterministic (priority/score order),
   not dependent on Go's randomized map iteration.
8. Three or more editions of the same work in one bucket — each target fills independently from a
   stable snapshot; no cascading double-enrichment.
9. Single-edition bucket (only one real result for a title) — no-op, no panic on an
   empty/singleton slice.

## Files to touch

- `internal/handlers/metadata_consolidate.go` — add `enrichAcrossEditions` and its helpers (donor
  selection, language guard); call it from `consolidateResults` (currently ~line 245, between the
  `merged` build and the `sort.SliceStable` call).
- `internal/handlers/metadata.go` — add `EnrichedFields []string` to `BookMetadataResult` (near
  line 38).
- `internal/handlers/metadata_consolidate_test.go` — extend, following the existing
  `TestConsolidateResults_*` naming convention (e.g.
  `TestConsolidateResults_EnrichesSparseEditionFromRicherEdition`,
  `TestConsolidateResults_DoesNotCrossFillEditionSpecificFields`,
  `TestConsolidateResults_SkipsDescriptionOnLanguageMismatch`,
  `TestConsolidateResults_ExcludesEmptyTitleOrAuthorFromBucketing`), covering the edge cases
  above.

## Verification (once implemented)

- `pnpm nx test bookshelf-backend` (or `go test ./internal/handlers/... -run TestConsolidateResults -v`
  from `apps/bookshelf-backend`) — unit tests cover the logic directly, no live network calls
  needed since `consolidateResults` takes a `[]BookMetadataResult` slice.
- Manual check: run the backend (`pnpm nx serve bookshelf-backend`, or `DB_PATH=... go run
./cmd/server`) and hit `GET /books/metadata/search?q=<a title known to have multiple editions>`
  with a valid bearer token; confirm a previously-bare result now carries a cover/description and
  that `EnrichedFields` is populated only for the fields actually filled.
- `pnpm nx lint bookshelf-backend` (golangci-lint — `gocognit`, `revive`, etc. all apply to any
  new helper functions, per this app's `CLAUDE.md`).
