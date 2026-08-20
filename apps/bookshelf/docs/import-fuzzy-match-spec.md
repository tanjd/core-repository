# Import Fuzzy Matching — spec

**Status:** Approved for build · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `Book`, `Copy`, the existing `/copies/mine/import` preview-then-confirm flow

Add a normalized title/author fallback to book import's catalog matching, so a row with no
OpenLibrary key, Google Books ID, or matching ISBN still gets a chance to be recognized as a book
already on the shelf — surfaced to the member for confirmation, never merged automatically.

## Why now

Import (`POST /copies/mine/import/preview` + `POST /copies/mine/import`) already dedupes each row
against the catalog via `findExistingBook`: OpenLibrary key → Google Books ID → ISBN (only if both
keys are absent). A row that misses all three — common for a hand-edited file, an older export
predating one of these fields, or a plain ISBN typo — always falls through to creating a brand new
`Book`, even when the title and author are really the same book already in the catalog. That
produces duplicate catalog entries the community then has to notice and clean up by hand.

## Goals

- A row with no strong external key, whose title+author normalizes to match an existing catalog
  book, is flagged as a **possible match** in the import preview rather than silently becoming a
  new book.
- The member decides, per flagged row, whether to attach their copy to the existing book or add it
  as a new one — the system never merges on their behalf.
- The default, if a member takes no action on a flagged row, is to create a new book — matching is
  opt-in, since two different books can legitimately share a title and author (reprints, unrelated
  same-titled works), and a wrong auto-merge is a hard-to-notice mistake.

## Non-goals (v1)

- No new database table or migration — this is a classification-time enhancement to the existing
  import handler, not a new persisted concept.
- No fuzzy matching beyond normalized title+author equality (lowercase, punctuation stripped,
  whitespace collapsed) — no edit-distance/typo tolerance, no partial/substring matching.
- No change to the existing exact-key matching behavior (`match_existing_book` via OL
  key/Google Books ID/ISBN) — it keeps auto-applying exactly as today. This only adds a new,
  weaker tier below it.
- No UI for resolving a possible match outside the import dialog (e.g. no separate "merge
  duplicate books" admin tool) — out of scope, and a different problem (that would operate on
  books that already exist as separate catalog entries, not on an in-flight import).

## How matching works

Reuses `normalizeTitleAuthor` (`apps/bookshelf-backend/internal/handlers/metadata_consolidate.go`),
the same normalization already used to dedupe results from Open Library/Google Books/BookBrainz
during manual "add a book" search — now also applied against existing `Book` rows.

1. `findExistingBook` runs first, unchanged (OL key → Google Books ID → ISBN).
2. On a miss, a new `findFuzzyMatch(title, author)` step fetches the catalog (every `Book` with at
   least one copy — orphaned, copy-less books are already deleted elsewhere, so nothing stale is
   matched against) and compares each candidate's normalized title+author key to the row's. The
   first exact key match wins.
3. No match at either stage → row is classified `create_book`, same as today.

At this app's scale (a single self-hosted community's catalog), comparing in Go against the full
book list is simple and fast enough — no new repository method, index, or migration needed.

## API surface (extends the existing import endpoints)

Both `POST /copies/mine/import/preview` and `POST /copies/mine/import` keep their existing
request/response shape, extended:

| Change                      | Where                                   | Behavior                                                                                                                                                          |
| --------------------------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| New action `possible_match` | `importRowResult.action`                | Row's title+author matched an existing book; not auto-applied.                                                                                                    |
| Candidate surfaced          | `importRowResult`                       | `matched_book_id`/`matched_book_title`/`matched_book_author`, populated for both `match_existing_book` and `possible_match`.                                      |
| New summary count           | `importSummary`                         | `possible_matches`, alongside the existing `books_created`/`books_matched`/`copies_created`/`skipped`.                                                            |
| Per-row decisions on commit | `POST /copies/mine/import` request body | Optional `decisions` map, 1-based row number → `"accept_match"` \| `"create_new"`, resolving each `possible_match` row. Missing entry defaults to `"create_new"`. |

Preview and commit both re-parse the same raw file content from scratch on every call (no
server-side session between the two requests, matching the existing pattern) — decisions travel
with the commit call itself rather than being looked up from a stored preview, so the frontend
must resend the same file content plus the accumulated decisions map together.

## Open decisions

**Decision 1 — safe default on no decision.** An unresolved `possible_match` row commits as
`create_new`, not `accept_match`. A missed click should never silently merge two books; a missed
click producing an extra catalog entry is a much smaller, more visible mistake (easy to spot and
delete) than an accidental merge (silently loses the distinction between two books once copies are
attached).

**Decision 2 — ambiguous multiple candidates.** If more than one existing book normalizes to the
same title+author key (a pre-existing near-duplicate in the catalog), `findFuzzyMatch` returns the
first one found rather than erroring or surfacing a picker. Treated as an edge case not worth
extra UI for v1 — the member can still choose "add as new" if the suggested candidate looks wrong.

## Build order

1. Backend: `findFuzzyMatch` helper, `possible_match` action, `importRowResult`/`importSummary`
   extensions, `decisions` input, `commitImportPlan` resolution logic.
2. Backend tests: extend the existing `copies_import_test.go` suites for classification, preview,
   and commit (both decision branches, plus the ambiguous-candidate and empty-normalized-key edge
   cases).
3. Frontend: `api.ts` type/param extensions, then the My Books import dialog — candidate display
   and accept/create-new toggle per flagged row, decisions map threaded into the commit call.
4. End-to-end coverage in `apps/bookshelf-e2e`, extending the existing import/export round-trip
   spec.

## How we'll know it's working

Fewer duplicate catalog entries showing up from community members importing older or hand-edited
export files — the thing this was built to prevent. Watching whether members actually use "accept
match" (vs. always defaulting to "add as new" out of caution) is worth a look post-launch too; if
nobody ever accepts a suggested match, the normalization or candidate set may need revisiting.

## Implementation notes

**Backend** (`apps/bookshelf-backend`):

- `internal/handlers/copies_import.go`:
  - New `actionPossibleMatch importAction = "possible_match"` constant.
  - `importRowResult` gains `MatchedBookID *uint`, `MatchedBookTitle string`,
    `MatchedBookAuthor string`.
  - `importSummary` gains `PossibleMatches int`.
  - `importInput.Body` gains `Decisions map[string]string` (row number string → `"accept_match"` /
    `"create_new"`).
  - `classifyImportRow`: on `findExistingBook` miss, call `findFuzzyMatch` before falling back to
    `actionCreateBook`.
  - New `findFuzzyMatch(books repository.BookRepository, title, author string) (*models.Book, error)`:
    `books.List("", "", false)`, then compare `normalizeTitleAuthor` keys; guard against the
    empty/`|`-only key case the same way `deduplicateIntoGroups`'s `findExistingGroup` already does
    in `metadata_consolidate.go`.
  - `commitImportPlan`: branch on `plan.action == actionPossibleMatch` using the row's decision
    (looked up by 1-based row number), defaulting to `actionCreateBook` behavior when absent;
    report the actual resolved action in the response.
  - `tallyImportResult`: handle `possible_match` (tallies `PossibleMatches`; the resolved
    create/match action still tallies `BooksCreated`/`BooksMatched`/`CopiesCreated` as usual).
- `internal/handlers/copies_import_test.go`: extend `TestCopyHandler_ClassifyImportRow`,
  `TestCopyHandler_PreviewImportBooks`, `TestCopyHandler_ImportBooks` per Build order step 2.

**Frontend** (`apps/bookshelf`):

- `src/lib/api.ts`: `ImportRowAction` gains `"possible_match"`; `ImportRowResult` gains
  `matched_book_id?`, `matched_book_title?`, `matched_book_author?`; `ImportSummary` gains
  `possible_matches`; `importBooks` gains an optional `decisions` param sent in the request body.
- `src/app/my-books/page.tsx`:
  - `importActionVariant`/`importActionLabel` gain a `possible_match` entry, visually distinct
    from both `create_book` (`success`) and `match_existing_book` (`outline`) — e.g. `secondary` /
    "Possible match".
  - `importSummaryText` gains a `possible_matches` clause.
  - New per-row decisions state (`Record<number, "accept_match" | "create_new">`), reset alongside
    the existing `importPreview`/`importResult` resets in `resetImportDialog` and
    `handleImportFileSelected`.
  - Preview list gains, per `possible_match` row: the candidate's title/author and a toggle
    writing into the decisions state.
  - `handleImportConfirm` passes the accumulated decisions map into `api.importBooks`.

### Critical files

- `internal/handlers/copies_import.go`, `internal/handlers/copies_import_test.go`
- `internal/handlers/metadata_consolidate.go` (reused `normalizeTitleAuthor`, matching pattern)
- `internal/handlers/books.go` (`findExistingBook`, the precedence this extends)
- `src/lib/api.ts`, `src/app/my-books/page.tsx`

### Verification

- Backend: `pnpm nx test bookshelf-backend`.
- Frontend: no dedicated unit tests for this page today — verify manually via
  `pnpm nx dev bookshelf` against a local backend, importing a file with a title/author match and
  confirming both the accept-match and add-as-new paths.
- End-to-end: extend `apps/bookshelf-e2e/src/import-export-books.spec.ts` per Build order step 4,
  `pnpm nx e2e bookshelf-e2e`.
- Full gate: `pnpm nx affected -t lint test` before merging.
