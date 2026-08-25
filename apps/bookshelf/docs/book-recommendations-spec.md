# Book Recommendations (Topical) — spec

**Status:** Planned — not approved for build · **Gated on:**
`community-reading-activity-spec.md` shipping + ~1 month of observed usage · **Scope:**
`apps/bookshelf` + `apps/bookshelf-backend` · **Depends on:** `Book`, `Copy`, `LoanRequest`,
`Notification`

Let members endorse books they've actually read with **topical tags** (e.g. `prayer`,
`parenting`, `evangelism`) — so the catalog can answer _"what's good on X?"_ instead of forcing a
reader to browse a global "popular" list and hope.

This spec is deliberately **not approved for build.** It replaces the earlier `book-reviews-spec.md`
(now deprecated), which answered a different question (per-book star ratings). It sits behind
Feature A (`community-reading-activity-spec.md`) and is only revisited if that feature ships and
members still ask for topical discovery afterward.

## Why this shape, not per-book star ratings

The original review spec optimised for _"is this specific book good?"_ (Goodreads-style). Members
actually asked _"what's being read on X?"_ and _"what would people recommend?"_ Star ratings
answer the first, tags answer the second — and the second is what was asked. Concretely, tags
have three advantages here:

1. **Signal survives small N.** On a catalog with 1–2 endorsements per book, a 4.5-star average
   is noise; "2 members recommend this for `parenting`" is still information.
2. **Discovery pivot for free.** Once tags exist, `/recommended/[tag]` is a route away — no
   separate "categorize the catalog" project.
3. **Matches how people actually recommend books in a small community.** "You should read this for
   X" is the natural sentence; "I rate this 4/5" is not.

Explicit trade-off: this loses the "definitive rating" mental model. That's fine — the product's
job (per `apps/bookshelf-backend/CLAUDE.md`) is exchange, not book criticism; link out to Google
Books for a rating.

## Preconditions before this gets approved

Do **not** start this until all four are true:

1. Feature A has shipped and been live for at least one month.
2. `sort=popular` usage in the catalog is non-trivial (rough bar: >5% of catalog visits with an
   explicit sort change).
3. Members have separately asked for _topical_ discovery ("books on X") after Feature A shipped —
   not just "reviews" or "ratings" in the abstract.
4. `CLAUDE.md`'s product-scope guardrail is either amended to explicitly allow member-provided
   endorsement metadata, or a clear carve-out is documented (the "we allow endorsement but not
   review/rating" line — see the "Scope-guardrail decision" section below).

If any of these is missing, defer.

## Goals

- A member who owns or has borrowed a book can endorse it with (a) an optional recommend/skip
  boolean and (b) 0–5 free-text tags.
- A book's detail page shows aggregate endorsements grouped by tag ("3 members recommend this
  for `prayer`").
- A `/recommended/[tag]` page lists books ordered by endorsement count for that tag.
- One endorsement per member per book, editable, not anonymous.

## Non-goals (v1 of this feature)

- **Star ratings.** Explicit no. See the reasoning above.
- **Free-text review bodies / comments.** Explicit no — brings moderation load, gives no discovery
  lift, and duplicates what Google Books already does well.
- **Curated / admin-owned tag taxonomy.** Tags are member-provided free text, normalised
  (lowercase, trim, collapse whitespace, `-` for spaces). Curation can come later if the tag
  space fragments (see "Open questions" below).
- **Notification-to-owners when a new endorsement lands.** The wishlist-workflow-style notify path
  is out for v1 — it adds surface without a clear benefit for endorsements (unlike loan requests,
  which need attention). Revisit if members ask for it.
- **Photo attachments, upvoting endorsements, moderation queue.**

## Who can endorse what

Same eligibility rule as the deprecated review spec — a recommendation should mean the person
actually read the book:

- **Eligible:** the member owns a copy of the book, **or** has an `accepted`/`returned`
  `LoanRequest` against a copy of it.
- **One endorsement per member per book.** Re-submitting edits the existing row rather than
  creating a second one.
- **Only the author can edit or delete.** No admin override in v1.

## Data model (proposed)

New `Recommendation` table. No changes to `Notification` for v1 (see "Non-goals" above — no
notify workflow).

| Field                       | Type     | Notes                                                                      |
| --------------------------- | -------- | -------------------------------------------------------------------------- |
| `id`                        | `uint`   | Primary key.                                                               |
| `book_id`                   | `uint`   | FK → `books.id`. Part of a unique pair with recommender.                   |
| `recommender_id`            | `uint`   | FK → `users.id`.                                                           |
| `recommends`                | `bool`   | `true` = recommend, `false` = read but wouldn't recommend. Default `true`. |
| `tags`                      | `string` | Comma-separated, normalised, 0–5 entries. See "Tag storage" below.         |
| `created_at` / `updated_at` | `time`   | Standard timestamps.                                                       |

Unique index on `(book_id, recommender_id)`.

**Tag storage — decision needed at approval time:**

- _Option A (recommended for v1):_ `tags` as a comma-separated string on `Recommendation`. Simple,
  matches SQLite's strengths, cheap to add. Aggregation is `SELECT tag, COUNT(*) ...` after a
  `json_each`-style split, or done in Go after fetch. Downside: no referential integrity, no
  cheap "list all known tags" query.
- _Option B:_ Separate `tag` table + `recommendation_tags` join table. Cleaner, standard shape,
  but three tables and two migrations for a feature we're not sure will earn its keep. Overkill
  for v1.

Pick A unless the tag surface grows unexpectedly complex during v1.

## API surface (proposed)

| Route                              | Auth        | Behavior                                                                         |
| ---------------------------------- | ----------- | -------------------------------------------------------------------------------- |
| `GET /books/{id}/recommendations`  | Public      | List a book's endorsements, newest first — recommender name + recommends + tags. |
| `POST /books/{id}/recommendations` | Required    | Upsert an endorsement. 403 if ineligible.                                        |
| `PATCH /recommendations/{id}`      | Author only | Edit `recommends` / `tags`.                                                      |
| `DELETE /recommendations/{id}`     | Author only | Remove the endorsement.                                                          |
| `GET /recommendations/tags`        | Public      | List distinct tags with counts (for the tag cloud / discovery entrypoint).       |
| `GET /recommendations/tags/{tag}`  | Public      | List books endorsed with `{tag}`, ordered by endorsement count DESC, then title. |

`GET /books` and `GET /books/{id}` responses gain `recommendation_count` (int) and `top_tags`
(top 3 tags by frequency for that book, or empty). The single-book response additionally carries
`you_can_recommend` (bool) and `your_recommendation` (nullable), so the frontend doesn't need a
second round trip to decide whether to render the endorse form.

## Frontend surface (proposed)

- `src/components/TagChips.tsx` (new): render a list of tags as small chips (reused on book
  detail, tag list, endorsement cards). No shadcn primitive — plain styled `<span>`s.
- `src/components/RecommendSection.tsx` (new): fetches `getBookRecommendations`, shows the count +
  top tags + list of endorsers, and — gated on `you_can_recommend` — an "Endorse this book"
  dialog with a recommend/skip toggle and a tag input (comma-separated, max 5, live-normalised).
- `src/app/recommended/[tag]/page.tsx` (new): lists books for a tag. Reuses `BookCard`.
- `src/app/catalog/page.tsx`: adds a new sort option `Most Recommended`. Optionally, a horizontal
  tag-cloud row above the results (top N tags, click → `/recommended/[tag]`).
- `src/app/catalog/[bookId]/page.tsx`: add `<RecommendSection />` below the "Copies" section. If
  Feature A shipped, `<RecommendSection />` sits below its borrow/waitlist stats row.
- `src/lib/types.ts`: `Recommendation` type; extend `Book` with `recommendation_count?`,
  `top_tags?`, `you_can_recommend?`, `your_recommendation?`.

## Backend surface (proposed)

- `internal/models/models.go`: new `Recommendation` struct with the fields above.
- Migration `000010_create_recommendations.{up,down}.sql` (next available number at approval
  time — `000009` is the current tail, but new migrations may land before this feature is
  approved; check `internal/db/migrations/` at that time). Table + unique `(book_id,
recommender_id)` index.
- `internal/repository/repository.go` + `internal/repository/gorm/recommendation_repo.go` (new):
  `Create`, `Save`, `Delete`, `GetByID`, `FindByBookAndRecommender`, `ListByBookID`,
  `CountByBookBatch(bookIDs) (map[uint]int64, error)`, `TopTagsByBookBatch(bookIDs, k) (map[uint][]string, error)`,
  `ListDistinctTagsWithCounts()`, `ListBooksByTag(tag string) ([]models.Book, error)`,
  `CanUserRecommend(bookID, userID) (bool, error)` (same eligibility check the review spec had —
  join through `Copy` for ownership OR through `Copy`+`LoanRequest` for a completed loan),
  `DeleteByBookID(bookID) error` (for the orphaned-book cleanup path).
- `internal/handlers/recommendations.go` (new): the six routes above.
- `internal/handlers/books.go`: `bookResponse` gains `RecommendationCount int64` + `TopTags []string`;
  `getBook` additionally gains `YouCanRecommend bool` and `YourRecommendation *models.Recommendation`.
- `internal/handlers/copies.go`: `maybeDeleteOrphanedBook` calls
  `recommendations.DeleteByBookID(bookID)` alongside the existing `wishlists.ClearFulfilledBookID`
  call.
- `cmd/server/main.go`: wire `recommendationRepo` into `NewBookHandler`, construct and register
  `RecommendationHandler`.

## Scope-guardrail decision

Both `apps/bookshelf/CLAUDE.md` and `apps/bookshelf-backend/CLAUDE.md` explicitly warn against
_"ratings, reviews, 'more like this', richer descriptions"_ as out-of-scope book-discovery
features. Endorsements-with-tags is closer to _member-provided metadata about a book they've
read_ than to _external book criticism_ — but it's still a metadata surface, and the guardrail
should be updated at approval time rather than quietly ignored. Proposed amendment to both files:

> Ratings, reviews, and long-form book criticism remain out of scope — link out to Google Books.
> Member endorsements of a book they've actually read (recommend/skip + short topical tags) are
> in scope, because they surface community reading behaviour (adjacent to the exchange flow) and
> support discovery of _"what's being read here"_ without duplicating an external site's job.

If the amendment feels too broad, this feature isn't approved yet.

## Open questions to resolve at approval time

- **Tag hygiene.** Left to natural convergence in v1 (normalisation + surface the top-N tag list
  prominently so members reuse existing tags). If tag fragmentation is bad after a month, add
  admin merge/rename tools then, not now.
- **Notify workflow.** Explicitly deferred (see Non-goals). Revisit if members ask.
- **Global "top tags" limit.** The tag cloud on the catalog page probably wants a hard cap (top
  20?) with a "see all" affordance — decide at implementation time.
- **Does this warrant its own top-level nav tab?** Probably not — `/recommended` is reachable
  via tag chips on the catalog and detail pages. If usage is high, promote to a nav tab; if not,
  keep it as a discovery-only URL.

## Build order (when approved)

1. Data model + migration.
2. Repository layer + eligibility check + batch aggregates. Tests alongside existing repo suite.
3. API handlers.
4. Frontend — tag chips, recommend dialog, `/recommended/[tag]` page, catalog tag cloud +
   `sort=recommended`.
5. End-to-end coverage — borrow → return → recommend with tags → detail-page aggregate → tag
   listing page.

## How we'll know it's working

Ship gates for approval (repeated here so they're not lost):

- Ratio of eligible members who leave at least one endorsement ≥ some threshold (pick at
  approval; ~15% for a first-month bar).
- Distinct tag count grows past the trivial (>10 tags across the catalog) within a month —
  otherwise the tag pivot didn't earn its shape.
- `/recommended/[tag]` page views are non-zero and repeat — otherwise members aren't actually
  using the discovery pivot, and this could've been a simpler recommend/skip boolean.

If any of these fails a month post-launch, prune the feature back rather than layering more on
top: the recommend/skip boolean without tags may still be worth keeping; the tag surface may be
worth removing.
