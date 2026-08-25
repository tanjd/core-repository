# Book Reviews — spec (SUPERSEDED)

**Status:** Superseded — do not build · **Superseded on:** 2026-08-25 · **Replaced by:**
[`community-reading-activity-spec.md`](./community-reading-activity-spec.md) (approved) and
[`book-recommendations-spec.md`](./book-recommendations-spec.md) (planned, not yet approved).

## Why this was dropped

This spec proposed a per-book 1–5 star rating + free-text review system, modelled loosely on
Goodreads/Amazon. On review it had three problems, in decreasing order of importance:

1. **Wrong question.** Members had asked _"what's being read in the community?"_ and _"what would
   people recommend for X?"_ — not _"is this specific book good?"_ Star ratings answer the third
   question; the first two are answered better by (a) surfacing existing loan-activity data
   (Feature A) and (b) tag-based endorsements (Feature B), respectively.
2. **Violated the product-scope guardrail** in both `apps/bookshelf/CLAUDE.md` and
   `apps/bookshelf-backend/CLAUDE.md`, which explicitly parked ratings/reviews as out-of-scope
   book-discovery features. The spec didn't propose amending the guardrail; it just ignored it.
3. **Signal-poor at this catalog's scale.** With small N per book (dozens–low hundreds of books,
   handful of borrowers each), star averages cluster at 4–5 with no separation between titles —
   the spec's own "How we'll know it's working" section anticipated this outcome. Building a
   feature that we predicted might not carry signal was a weak bet.

## What replaced it

- **[Community Reading Activity](./community-reading-activity-spec.md)** — approved to build.
  Surfaces borrow counts and waitlist depth on each book, plus a `sort=popular` option on the
  catalog. Zero new data collection, no new tables, no user-generated content.
- **[Book Recommendations (Topical)](./book-recommendations-spec.md)** — planned but not
  approved. Would let members endorse read books with topical tags (`prayer`, `parenting`, etc.)
  rather than 1–5 stars, giving the `/recommended/[tag]` discovery surface members were actually
  asking for. Gated on Feature A shipping and being observed for a month first.

The original content of this spec is preserved in git history if the detailed implementation
plan is ever needed as a reference.
