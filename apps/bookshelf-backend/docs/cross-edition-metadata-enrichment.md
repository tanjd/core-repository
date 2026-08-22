# Spec: cross-edition metadata enrichment in search results

Status: **implemented**. Also required (and applied) fixing `deduplicateIntoGroups`'s
title+author fallback, which previously fired whenever an ISBN lookup missed rather than only
when the incoming result itself had no ISBN — silently merging distinct-ISBN editions with
matching title+author into one row before `enrichAcrossEditions` could ever see them as separate
entries. See `findExistingGroup` in `internal/handlers/metadata_consolidate.go`.

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
  one `/books/metadata/search` call, before caching/returning. The enrichment _pass itself_ does
  no DB writes — but its _output_ is not fully insulated from persistence: if a user selects an
  enriched result and submits it via `createBook`, the backfilled value is copied straight onto
  the new `Book` row (`createBook` copies fields verbatim off the submitted result, no re-merge).
  So a wrong backfill isn't purely a transient display concern — see the language guard and the
  narrowed field scope below, both of which exist to keep that risk small.
- **Out of scope**: backfilling already-persisted `Book` catalog rows across editions, and
  actually merging two `Book` rows into one (repointing `Copy`/`WishlistRequest.FulfilledBookID`,
  etc.) remain two distinct problems from what's specced here. The former — persisted-row
  `Description` backfill via a scheduled reconciliation job — is no longer deferred: see
  [`catalog-description-reconciliation-job.md`](./catalog-description-reconciliation-job.md) for
  that design. Row-level merging is still unaddressed.
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

### Field scope: `Description` only

Only `Description` is cross-filled — `CoverURL` was considered and deliberately dropped:

- `CoverURL` already has a graceful fallback in the frontend (`BookCoverFallback`'s gradient
  placeholder, shipped in PR #60) — a sparse edition without its own cover doesn't look broken
  today, it looks intentional. The gap backfilling would close is smaller than it first appears.
- A wrong/mismatched cover is a sharper problem in this app than in ordinary e-commerce browsing:
  this is peer-to-peer physical lending, not a storefront where "photo may vary" is an accepted
  norm. A member requesting a book based on a pictured cover, then receiving a different physical
  printing at handoff, is a small but real trust/expectation mismatch. `Description` carries no
  equivalent physical-handoff risk — worst case it's mildly inaccurate text about a book nobody
  has picked up yet.
- `Description` has no existing fallback (an empty field just renders blank) and is the field the
  original problem statement's pain point centers on, so keeping it preserves the change's core
  value even with `CoverURL` dropped.

`Publisher`, `PublishedDate`, and `PageCount` are genuinely **edition-specific** — a different
ISBN can legitimately have a different page count or publisher — so those must **not** be
cross-filled, or a sparse edition would silently display factually wrong specifics for the ISBN
the user is about to pick. `Title`, `Author`, `ISBN`, `OLKey`, `GoogleBooksID`, `BookBrainzID` are
never touched by this pass — copying identity keys across editions would corrupt
`findExistingBook`'s downstream dedupe (`internal/handlers/books.go`) if the enriched result is
later used to create a `Book`.

### Language guard on `Description`

Skip a candidate donor whose `Language` is non-empty and differs (case-insensitive) from the
target's non-empty `Language` — otherwise a French edition's description could get attached to an
English edition's search result. If either side's `Language` is unset, proceed (best-effort; most
results in practice are single-market/English).

### Transparency field — mandatory in the UI

Add `EnrichedFields []string` (`json:"enriched_fields,omitempty"`) to `BookMetadataResult`
(`internal/handlers/metadata.go`, near line 38), listing which fields were filled from a
different edition. Additive, `omitempty`, doesn't break existing struct-equality tests.

Unlike the original draft of this spec, labeling is **required**, not optional polish: any
displayed `Description` that was backfilled must be visibly marked (e.g. "description from
another edition") in the UI, both wherever a single result renders and in the collapsed/expanded
bucketed views described below. A silently-borrowed description undermines the exact judgment
call a user needs to make when comparing editions to pick one — labeled, it's a transparent
fallback; unlabeled, it's an unqualified claim about that specific edition that might be false
(see the "Why not just merge automatically" risk this spec already accepts elsewhere).

Since only `Description` is ever backfilled now (`CoverURL` was dropped — see field scope above),
`EnrichedFields` in practice only ever contains `"description"`. A future implementer may find the
`[]string` shape over-general for a one-field list and prefer a single `DescriptionEnriched bool`
instead — left as an open implementation choice, not a decision made here.

## Display: bucketing editions in the frontend

Backfill alone doesn't solve the original complaint on its own: two now-similarly-rich rows for
the same work is _more_ confusing to browse than one sparse and one rich row, not less — the user
still can't tell why two near-identical results exist or which one to pick. This section adds a
purely-additive, display-only bucketing layer on top of backfill, decided during this spec's
brainstorm.

**Terminology, to keep three similar-sounding operations distinct**: this doc's existing
mechanism, `deduplicateIntoGroups`/`mergeGroup`, **groups** and then truly **merges** multi-source
hits for the _same edition_ into one combined row — that already happens today, upstream, and
nothing here changes it. `enrichAcrossEditions` (above) **backfills** one empty field on a sparse
_edition_ from a richer sibling edition — still no combining, just one field copied onto an
otherwise-untouched result. This section **buckets** — clusters already-distinct, already-merged
edition results _for display only_; it never combines two editions' data into one object, and
"bucket" is used throughout this section specifically so it doesn't read as a fourth flavor of
"group" or "merge."

- **New field**: `WorkKey string` (`json:"work_key,omitempty"`) added to `BookMetadataResult`
  (`internal/handlers/metadata.go`), computed via the same `normalizeTitleAuthor(r.Title,
r.Author)` key `enrichAcrossEditions` already buckets by — stamp it onto each merged result
  right where buckets are already computed, rather than discarding the key after use. Entries with
  empty `Title` or `Author` get an empty `WorkKey` (same exclusion rule as backfill bucketing) —
  the frontend must never bucket by an empty key.
- **No response shape change**: `consolidateResults` keeps returning the same flat, score-sorted
  `[]BookMetadataResult` it does today — bucketing is entirely a frontend concern, so existing
  backend tests (`TestConsolidateResults_RanksByCompletenessThenTitle`,
  `TestConsolidateResults_KeepsDistinctBooksSeparate`, etc.) are unaffected by this section.
- **Frontend bucketing** (`apps/bookshelf/src/app/share/components/MetadataSearchStep.tsx`):
  bucket the received array by `work_key`; a non-empty key with more than one member renders as
  one bucketed card (collapsed by default) instead of `N` separate compact rows. Empty keys and
  singleton buckets render exactly as today — no behavior change for those.
- **Collapsed card**: shows the first member of that bucket in array order — already the
  highest-scored, since the backend pre-sorts by score descending, so no extra client-side scoring
  logic is needed — using its own `CoverURL` and its (possibly backfilled + labeled) `Description`.
- **Expanded view**: lists every edition in the bucket individually, each with its own
  `Publisher`/`PublishedDate`/`PageCount`/`ISBN` (never touched by backfill or bucketing) — this is
  where the user picks the exact edition to add. Selecting one flows into `handleSelectResult` /
  `createBook` exactly as today, unchanged — bucketing only changes how results are browsed, never
  what gets submitted.
- `WorkKey` equality is purely `normalizeTitleAuthor` equality, carrying the same known
  false-positive risk already accepted elsewhere in this codebase (an omnibus/anthology edition
  can share a normalized key with a single-volume edition — see
  `TestCreateBook_DoesNotDedupByISBN_WhenOLKeyProvided` in `books_test.go` for existing precedent
  of this exact risk being called out). This is why the mandatory label above matters even at the
  bucket-display layer, not only for backfill — a user comparing bucketed editions is exactly the
  moment they need to trust which facts are genuinely per-edition.

## Edge cases to cover (design + tests)

1. Two editions, same normalized title+author, different ISBN — sparse one gets `Description`
   filled from the richer one; the richer one is left untouched. `CoverURL` is never touched by
   this pass on either side.
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
10. A backfilled `Description` always carries the `EnrichedFields` marker — no code path may set
    the field without marking it
    (`TestConsolidateResults_EnrichedDescriptionIsAlwaysLabeled`).
11. `WorkKey` (see bucketing section below) is populated identically to the bucket key used for
    backfill — both derive from the same `normalizeTitleAuthor` call, so a result's bucket
    membership and its backfill eligibility are never inconsistent with each other.

## Build requirement: end-user help content

Once bucketing is live, add a short, non-technical explainer visible from the search results UI
(e.g. an info icon/tooltip near a bucketed card in `MetadataSearchStep.tsx`) — something like:
"We group listings for the same book and fill in missing details from other editions when we're
confident it's the same title." Keep it in plain language for community members, not the internal
mechanics (`WorkKey`, `scoreResult`, source priority) documented above — this note is a
requirement for whoever implements this spec, not something to write speculatively now, since the
feature it describes doesn't exist yet.

## Files to touch

**Backend** (`apps/bookshelf-backend`):

- `internal/handlers/metadata_consolidate.go` — add `enrichAcrossEditions` and its helpers (donor
  selection, language guard); call it from `consolidateResults` (currently ~line 245, between the
  `merged` build and the `sort.SliceStable` call). Also stamp `WorkKey` onto each merged
  result at the same bucketing point.
- `internal/handlers/metadata.go` — add `EnrichedFields []string` and `WorkKey string` to
  `BookMetadataResult` (near line 38).
- `internal/handlers/metadata_consolidate_test.go` — extend, following the existing
  `TestConsolidateResults_*` naming convention (e.g.
  `TestConsolidateResults_EnrichesSparseEditionFromRicherEdition`,
  `TestConsolidateResults_DoesNotCrossFillEditionSpecificFields`,
  `TestConsolidateResults_SkipsDescriptionOnLanguageMismatch`,
  `TestConsolidateResults_ExcludesEmptyTitleOrAuthorFromBucketing`,
  `TestConsolidateResults_EnrichedDescriptionIsAlwaysLabeled`,
  `TestConsolidateResults_WorkKeyMatchesBucketKey`), covering the edge cases above.

**Frontend** (`apps/bookshelf`):

- `src/lib/types.ts` — add `work_key?` and `enriched_fields?` to the `BookMetadataResult`
  type (lines 169-183).
- `src/app/share/components/MetadataSearchStep.tsx` — bucket results by `work_key` before
  rendering; add the collapsed/expanded bucketed-card UI and the mandatory "from another edition"
  label on any result whose `enriched_fields` includes `"description"`. No dedicated unit tests
  exist for this component today (per `import-fuzzy-match-spec.md`'s note on the sibling
  `my-books` page) — verify manually per below.

## Verification (once implemented)

- `pnpm nx test bookshelf-backend` (or `go test ./internal/handlers/... -run TestConsolidateResults -v`
  from `apps/bookshelf-backend`) — unit tests cover the logic directly, no live network calls
  needed since `consolidateResults` takes a `[]BookMetadataResult` slice.
- Manual check: run the backend (`pnpm nx serve bookshelf-backend`, or `DB_PATH=... go run
./cmd/server`) and hit `GET /books/metadata/search?q=<a title known to have multiple editions>`
  with a valid bearer token; confirm a previously-bare result now carries a `description` and that
  `enriched_fields`/`work_key` are populated correctly.
- Manual frontend check: `pnpm nx dev bookshelf` against a local backend, search a title known to
  have multiple real editions (different ISBNs), and confirm the bucketed card collapses/expands
  correctly, the labeled description shows when backfilled, and selecting any edition (not just
  the representative one) still flows correctly into `createBook`.
- `pnpm nx lint bookshelf-backend` (golangci-lint — `gocognit`, `revive`, etc. all apply to any
  new helper functions, per this app's `CLAUDE.md`).
- Full gate before merging: `pnpm nx affected -t lint test` (and `e2e` if `bookshelf-e2e` gets a
  new spec for this flow).
