# Spec: catalog description reconciliation job

Status: **implemented**.

Extends [`cross-edition-metadata-enrichment.md`](./cross-edition-metadata-enrichment.md), whose
`Description`-only backfill only ever touches the ephemeral `/books/metadata/search` response.
That doc originally deferred backfilling already-persisted `Book` catalog rows as a separate,
higher-stakes problem ("revisit separately if wanted") — this doc is that revisit, chosen over
deferring further because the app already has the exact infrastructure this needs.

## Problem

Two `Book` rows can already exist in the catalog for different editions of the same work (a
hardcover and a paperback, added at different times by different members), and one can be missing
a `Description` that the other has — the same gap the search-time spec fixes for ephemeral
results, just now already persisted. Unlike the search-time case, there's no natural moment (like
picking a search result) where this gets fixed — a sparse `Book` row just stays sparse forever
unless something goes back and looks.

## Why a scheduled job, not an on-create hook

The catalog already has a precedent for exactly this shape of maintenance:
`internal/services/scheduler.go`'s `cover-refresh` job re-downloads/re-caches cover images for
existing `Book` rows on a schedule, silently rewriting `CoverURL`, no per-item admin review — and
`backup` is a second job on the same `Scheduler`, registered via the generic
`RegisterJob(name, settingKey string, fallback time.Duration, run func(ctx context.Context) string)`.
Both are exposed through fully generic admin endpoints already, not job-specific ones:

- `GET /admin/jobs` (`internal/handlers/jobs.go`) — lists every registered job's `JobStatus`
  (`Running`, `Interval`, `LastRunAt`, `NextRunAt`, `LastResult`).
- `POST /admin/jobs/{job}/run` — triggers any registered job by name immediately
  (`Scheduler.TriggerNow`).

Adding a third job here means **reusing this generic machinery**, not building new admin routes —
the same reasoning that makes `cover-refresh` an accepted risk (scheduled, unsupervised, but
visible via `LastResult`/`LastRunAt` and re-runnable on demand) applies directly to a description
reconciliation job. No new approval/dry-run workflow is introduced; `GET /admin/jobs` already
gives an admin the same visibility into this job's last run and result that they'd have into
cover-refresh or backup today.

## Scope

- **In scope**: a new background job that scans existing `Book` rows, buckets them by
  work (same normalized title+author key the search-time spec uses), and backfills `Description`
  onto a sparse `Book` from a richer sibling — a real DB write, unlike the search-time pass.
- **Field scope: `Description` only** — same reasoning as the search-time spec (`CoverURL` has a
  frontend fallback and carries a sharper mismatch risk in a physical-lending context; see that
  doc's field-scope section). `Publisher`/`PublishedDate`/`PageCount`/identity keys are never
  touched here either, for the same reasons.
- **Backfill only**: never overwrites an already-populated `Description`.
- **Out of scope**: repointing `Copy` records or `WishlistRequest.FulfilledBookID` between
  editions, or actually merging two `Book` rows into one. This job only ever writes one column
  (`Description`, plus the transparency flag below) on rows that already exist — it never deletes,
  merges, or repoints anything. Row-level merging remains a distinct, unaddressed problem.

## Design

### Shared normalization helper needs extracting

`normalizeTitleAuthor` currently lives in `internal/handlers/metadata_consolidate.go`
(`internal/handlers` package). This job lives in `internal/services` and must not import
`internal/handlers` (wrong dependency direction — `handlers` already depends on `services`, not
the reverse). Extract `normalizeTitleAuthor` into a small shared package (e.g.
`internal/bookmatch`) imported by both `internal/handlers` (existing callers:
`deduplicateIntoGroups`, `enrichAcrossEditions`) and `internal/services` (this job) — pure
function move, no behavior change, update call sites in `metadata_consolidate.go` and (per the
`import-fuzzy-match-spec.md`) `copies_import.go`'s `findFuzzyMatch` too, so all three normalized-
title-author consumers share one implementation instead of drifting.

### The job itself

New `internal/services/description_reconciliation.go`, a service with a
`Run(ctx context.Context) string` method (matching the signature `RegisterJob` expects, same shape
as `BackupService.CreateSnapshot`):

1. Fetch the catalog via `books.List("", "", false)` — the same `BookRepository` call
   `findFuzzyMatch` already uses (`copies_import.go`), which already excludes copy-less books (the
   repository's `List`/`ListRecent` `EXISTS (SELECT 1 FROM copies ...)` filter), so nothing
   orphaned gets bucketed.
2. Bucket the result by `bookmatch.NormalizeTitleAuthor(book.Title, book.Author)`. Empty-key books
   (empty `Title` or `Author`) are excluded from bucketing, same rule as the search-time spec.
   Buckets of size 1 are no-ops.
3. Within a bucket with ≥2 books, **donor selection is simpler than the search-time spec's**:
   `Book` has no `Source`/`BookBrainzID` field to rank by (unlike `BookMetadataResult`), so there's
   no equivalent confidence signal to sort on. Instead: sort bucket members by `ID` ascending
   (creation order, deterministic — no map iteration involved), and the first member with a
   non-empty `Description` is the donor for every other member in the bucket missing one.
4. **Language guard**: same as the search-time spec — skip a donor whose `Language` is non-empty
   and differs (case-insensitive) from the target's non-empty `Language`.
5. **Never overwrite**: skip any target that already has a non-empty `Description`.
6. On backfill: set `Description = donor.Description`, `DescriptionEnriched = true`, persist via
   the existing `BookRepository` update path.
7. Return a human-readable summary string for `JobStatus.LastResult` (e.g. "backfilled 3 of 214
   books"), matching the convention other jobs already use.

**Idempotent by construction**: since it only ever writes to books with an empty `Description`, a
repeat run (whether scheduled or manually triggered) finds nothing left to do once the catalog is
caught up — safe to run as often as an admin likes, same as re-running cover-refresh.

### Transparency: persisted marker + catalog UI label

Unlike the search-time spec, `EnrichedFields` (an ephemeral response field) doesn't apply — the
marker needs to persist alongside the `Description` it describes. Add `DescriptionEnriched bool`
to `models.Book` (new migration `000010_add_book_description_enriched.up.sql` /
`.down.sql`, adding a `description_enriched` column, `NOT NULL DEFAULT FALSE`, following this
migration set's existing zero-padded sequence numbering).

Carrying forward the search-time spec's mandatory-label decision: `apps/bookshelf/src/app/catalog/[bookId]/page.tsx`
(the Book detail page, where `Description` renders) must show a visible note — "description from
another edition" — whenever `description_enriched` is true. This is the persisted-catalog
equivalent of the search-time UI's required label; skipping it here would silently break the same
transparency principle the search-time spec exists to uphold.

### Wiring into the scheduler

`cmd/server/main.go` (near the existing `scheduler.RegisterJob("backup", "backup_interval", ...)`
call, line ~109): add
`scheduler.RegisterJob("description-reconciliation", "description_reconciliation_interval", 24*time.Hour, reconciliationSvc.Run)`.
No new admin routes — `GET /admin/jobs` and `POST /admin/jobs/{job}/run` already handle any
registered job generically.

### Frontend: surfacing the new job in the existing Jobs page

`apps/bookshelf/src/app/admin/jobs/page.tsx` already renders every job `GET /admin/jobs` returns
generically via `ScheduledTaskCard`, keyed off two lookup maps — add one entry to each, no new
page or component needed:

- `JOB_META["description-reconciliation"]` — a label ("Description Reconciliation") and
  description ("Fills in missing book descriptions from other editions of the same book. Runs
  automatically on the configured interval.").
- `JOB_SETTING_KEYS["description-reconciliation"]` — `"description_reconciliation_interval"`, so
  the existing interval-editing UI (`INTERVAL_PRESETS`/`handleSaveInterval`) works for this job
  without new code.

Also: `apps/bookshelf/src/lib/types.ts` — add `description_enriched?: boolean` to the `Book` type.

## Edge cases to cover (design + tests)

1. Two books, same normalized title+author, different ISBN, one with `Description` — sparse one
   gets it backfilled and marked `DescriptionEnriched`; the donor is untouched.
2. Never overwrite an already-non-empty `Description`, even from a lower-`ID` donor.
3. Books excluded from bucketing when `Title` or `Author` is empty.
4. Description fill skipped when donor/target `Language` are both set and differ.
5. Three or more books in one bucket — deterministic donor selection by lowest `ID` with a
   non-empty `Description`, not dependent on query/map ordering.
6. Re-running the job after a previous run finds nothing left to backfill (idempotency).
7. A book with `DescriptionEnriched = true` that a member later edits directly (if a manual edit
   path exists) — out of scope to auto-clear the flag; left as a known gap, low-stakes (a stale
   "from another edition" label after a manual edit is a minor cosmetic mismatch, not a
   correctness problem).
8. `books.List("", "", false)` returning zero or one book total — no panic on an empty/singleton
   catalog.

## Files to touch

- `internal/bookmatch/` (new package) — `normalizeTitleAuthor` extracted here (exported as
  `NormalizeTitleAuthor`); update `internal/handlers/metadata_consolidate.go` and
  `internal/handlers/copies_import.go` to import from here instead of a local definition.
- `internal/services/description_reconciliation.go` (new) — the job itself.
- `internal/services/description_reconciliation_test.go` (new) — covering the edge cases above.
- `internal/models/models.go` — add `DescriptionEnriched bool` to `Book`.
- `internal/db/migrations/000010_add_book_description_enriched.up.sql` /
  `.down.sql` (new).
- `cmd/server/main.go` — register the job on the scheduler.
- `apps/bookshelf/src/lib/types.ts` — add `description_enriched?: boolean` to `Book`.
- `apps/bookshelf/src/app/admin/jobs/page.tsx` — `JOB_META`/`JOB_SETTING_KEYS` entries.
- `apps/bookshelf/src/app/catalog/[bookId]/page.tsx` — mandatory label when
  `description_enriched` is true.

## Verification (once implemented)

- `pnpm nx test bookshelf-backend` — unit tests for the new service, plus regression tests on
  `metadata_consolidate.go`/`copies_import.go` after the `normalizeTitleAuthor` extraction.
- Manual check: seed two `Book` rows (same title+author, different ISBN, one with a description),
  trigger the job via `POST /admin/jobs/description-reconciliation/run` (or the admin Jobs page's
  "Run Now"), confirm via `GET /admin/jobs` that `LastResult` reflects the backfill, and confirm
  the catalog detail page shows the "from another edition" label on the backfilled book.
- `pnpm nx lint bookshelf-backend`.
- Full gate before merging: `pnpm nx affected -t lint test`.
