# Book Recommendations — spec

**Status:** Approved for build · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** `Book`, `Copy`, `LoanRequest`

Let members give a book a lightweight thumbs-up — "I'd highly recommend this" — so the catalog can
surface well-liked books without building a star-rating or review system.

This spec is deliberately **not approved for build.** It replaces the earlier
`book-reviews-spec.md` (now deprecated), which answered a different question (per-book star
ratings). It sits behind Feature A (`community-reading-activity-spec.md`) and is only revisited if
that feature ships and members still ask for a "what's good" signal afterward.

Earlier drafts of this spec explored topical tags (`prayer`, `parenting`, ...) alongside the
thumbs-up, aiming at "what's good on X?" discovery. That's deliberately dropped for v1 — tags add
real surface (normalization, hygiene, a `/recommended/[tag]` route, aggregation-by-tag) for a
discovery need nobody's confirmed yet. If members ask for topical browsing after this ships,
revisit tags as a v2 addition on top of this simpler shape rather than building both at once.

This spec deliberately scopes to **intent and behavior**, not implementation. Table shapes, route
strings, file names, HTTP status codes, and index strategies are the implementer's call at build
time — the sections below constrain what the feature must _do_, not how it must be built. When a
choice below rules out a category of implementation (e.g. "deleted members' thumbs-ups don't
count"), that's a behavioral constraint on the resulting system, not a directive about which
query to write.

## Why this shape, not per-book star ratings

The original review spec optimised for _"is this specific book good?"_ (Goodreads-style), computed
as an average of 1–5 stars. Two problems with that on a small catalog:

1. **An average implies precision the data doesn't have.** On 1–2 ratings per book, "4.5 stars"
   reads as authoritative but is noise.
2. **It invites the wrong mental model.** The product's job (per `apps/bookshelf-backend/CLAUDE.md`)
   is exchange, not book criticism — link out to Google Books for a rating.

A thumbs-up count sidesteps both: it's a raw count, not an average, so it doesn't overstate its
own precision at low N ("2 members highly recommend this" reads as exactly what it is). And "I'd
recommend this" is the sentence people actually use in a small community — closer to a nod than a
review.

## Preconditions considered before approval

These were the four gates the feature had to clear before being approved for build — kept as a
record of what "ready" meant, so a future gated feature can borrow the shape:

1. Feature A has shipped and been live for at least one month.
2. `sort=popular` usage in the catalog is non-trivial (rough bar: >5% of catalog visits with an
   explicit sort change) — evidence members want a "what's good" view, not just alphabetical/newest.
3. Members have separately asked for a way to see which books are well-liked after Feature A
   shipped — not just "reviews" or "ratings" in the abstract.
4. `CLAUDE.md`'s product-scope guardrail is either amended to explicitly allow member-provided
   endorsement metadata, or a clear carve-out is documented (see "Scope-guardrail decision" below).

## Goals

- Any authenticated member can toggle a thumbs-up ("highly recommend") on a book.
- A book's detail page and every catalog card show the thumbs-up count.
- Catalog cards additionally reflect the current viewer's own state — the button appears
  "already recommended" when the viewer has recommended that book, so a member can toggle their
  own nod from the card without opening the detail page. This is the whole feature's UX bet: a
  thumbs-up is a nod, not a review, and a nod that costs three taps and a page load isn't a nod.
- The catalog gets a `Most Recommended` sort option.
- One thumbs-up per member per book. Toggling again removes it — there's no separate "edit" flow
  since there's nothing to edit besides on/off.

## Non-goals (v1 of this feature)

- **Star ratings.** Explicit no. See the reasoning above.
- **Topical tags / `/recommended/[tag]` discovery.** Deferred — see the intro. May become a v2 on
  top of this if members ask for it.
- **A negative signal ("wouldn't recommend").** This is a like, not a rating with two poles — not
  hearting a book says nothing. Modeled after a social "like" button, not a thumbs-up/down pair.
- **Free-text review bodies / comments.** Explicit no — brings moderation load, gives no discovery
  lift, and duplicates what Google Books already does well.
- **Anonymous thumbs-ups.** Unlike wishlist requests (which have an `anonymous` flag), a
  thumbs-up is an explicit endorsement — the recommender's identity is part of the signal. If
  members ask for anonymous recommends, revisit as v2, but the default expectation is your name
  attaches to your nod.
- **Notification-to-owners when a new thumbs-up lands.** The wishlist-workflow-style notify path is
  out for v1 — it adds surface without a clear benefit here (unlike loan requests, which need
  attention). Revisit if members ask for it.
- **Photo attachments, moderation queue, admin override.** There's no free text to moderate — a
  thumbs-up is just a member's own signal on their own account, so there's nothing here for an
  admin to remove that the member himself couldn't just un-toggle.

## Who can endorse what

Any authenticated member, no ownership or loan-history check. (An earlier draft gated this on
having owned/borrowed the book; dropped for the same reason as the review spec's version of that
rule — hard to enforce honestly, and it shrinks participation right when the feature needs it to
produce a useful signal.)

One thumbs-up per member per book, toggled on/off. No separate edit — re-tapping just removes it.

## Behaviors

### The thumbs-up itself

- **Toggling.** An authenticated member can add or remove their own thumbs-up on any book.
  Adding when already recommended is a no-op; removing when not recommended is a no-op. Two
  rapid taps in either direction never leave a book with duplicate rows, error the member out,
  or land in an inconsistent state — the second tap either lands as the toggle it looks like, or
  it's the no-op described above.
- **No admin override.** There is no admin surface for adding, removing, or moderating another
  member's thumbs-up. A thumbs-up is a member's own signal on their own account.

### What a book carries

- **A count.** Every book, everywhere it appears (catalog cards, detail page, list responses,
  detail responses), carries the number of members who currently recommend it. Members who have
  since been deleted from the community don't contribute to this count — see "Live-community
  signal" below.
- **A viewer-relative flag.** Every book response carries whether the current viewer has
  recommended it. For an anonymous viewer (no session), this is always false — nothing ever
  appears "recommended by you" without a logged-in you.

### Live-community signal

A book's count and facepile reflect the _current_ community, not historical membership. When a
member is deleted, their thumbs-ups fall out of every book's count and every facepile with them.
Rationale: the feature exists to answer "what would _this community_ recommend right now?" — an
ex-member's endorsement is stale by definition (unlike a completed loan under Feature A, which
is a factual historical event). This also keeps the count and the facepile trivially consistent
with each other: whatever the count says, the facepile can enumerate.

Whether this is implemented as an on-delete cascade, a filter-at-read, or something else is not
this spec's problem — the constraint is that the count and facepile must never include a member
who has been removed from the community.

### Catalog surface

- **Sort.** The catalog exposes a `Most Recommended` sort alongside its existing options. When
  active, books are ordered by thumbs-up count high to low; ties resolve by title so the order
  is stable across page loads.
- **Card.** Each catalog card shows the count and a toggle affordance that reflects the viewer's
  own state on that book (filled for "I recommend this", outline for "I don't"). Tapping the
  toggle recommends or un-recommends without leaving the catalog; the count and the toggle
  update immediately on tap and revert on failure (see "Failure UX" below).
- **Facepile stays off the card.** Cards remain at count + toggle only — no facepile at card
  density, consistent with this app's "cards over dense tables" mobile stance. Detail lives on
  the detail page.

### Detail-page surface

- **Toggle.** The detail page carries the same recommend affordance as the card, in the same
  state, wired to the same behavior. Tapping either surface toggles the same underlying thing.
- **Facepile.** Below the "Copies" section, the detail page shows who has recommended this
  book — the first few recommenders as small initials avatars, with an "and N others" tap
  target that opens a full names-only list of every recommender when there are more than fit.
  When nobody has recommended the book, the facepile renders nothing at all — no
  "recommended by nobody" empty state.
- **Empty-state coordination with Feature A.** If Feature A's borrow/waitlist stats row is
  already hidden (both counts zero) and no member has recommended this book yet, the entire
  community-signal area is absent from the detail page rather than leaving a lonely orphan
  component.

### Failure UX

Toggling is optimistic: the count and the toggle update the instant the member taps, before the
server confirms. If the request fails (network, session expired, server error), the UI reverts
to its pre-tap state and surfaces a non-blocking error message. The member is never left staring
at a count that says one thing while the server thinks another.

### Accessibility

The recommend button is icon-only on both card and detail-page surfaces. Its accessible name
discloses both its current state ("recommended" vs "not recommended") and the book it acts on,
so a screen-reader user hears "recommend _Deep Work_" rather than "button." The "and N others"
affordance on the facepile is a real focusable, keyboard-activatable control, not decorative
text.

## Scope-guardrail decision

Both `apps/bookshelf/CLAUDE.md` and `apps/bookshelf-backend/CLAUDE.md` explicitly warn against
_"ratings, reviews, 'more like this', richer descriptions"_ as out-of-scope book-discovery
features. A thumbs-up is closer to _a member's own lightweight signal on a book they're vouching
for_ than to _external book criticism_ — but it's still community-provided metadata, and the
guardrail should be updated at approval time rather than quietly ignored. Proposed amendment to
both files:

> Ratings, reviews, and long-form book criticism remain out of scope — link out to Google Books.
> A simple member "highly recommend this" thumbs-up is in scope, because it surfaces community
> reading behaviour (adjacent to the exchange flow) without implying a rating average or duplicating
> an external site's job.

If the amendment feels too broad, this feature isn't approved yet.

## Open questions — resolved

These were open during earlier drafts of this spec and are settled now; kept here so the reasoning
isn't lost:

- **Facepile avatar treatment:** reuse the app's existing initials-avatar look (the small
  circle-with-two-letters used on copy cards for owners) rather than inventing something new.
- **Facepile size:** show the first 3 recommenders, collapse the rest into "and N others."
- **Overflow behavior:** "and N others" is tappable/focusable, opening a simple names-only
  list — not just decorative text.
- **Card interactivity:** cards carry a live toggle reflecting the viewer's own state, not a
  count-only decoration. See "Catalog surface."
- **Deleted members:** their thumbs-ups don't count. See "Live-community signal."
- **Threshold badge ("Community Favorite"):** considered as an alternative to the facepile, not
  chosen for v1. Worth revisiting as a v1.1 addition if, post-launch, catalog browsing still seems
  to need a stronger "this is popular" signal than a bare count provides — but don't build it
  speculatively alongside v1.

## Open questions still to resolve at approval time

- **Notify workflow.** Explicitly deferred (see Non-goals). Revisit if members ask.
- **Does this warrant its own top-level nav tab or discovery page?** Probably not for a v1 that's
  just a count + sort — no dedicated `/recommended` route exists in this shape. If usage is high,
  a "Most Recommended" landing view could be added later without needing tags.
- **Topical tags as a v2.** If members ask for "what's good on X" after this ships, revisit the
  earlier tag design rather than bolting it on ad hoc — see the intro.

## How we'll know it's working

Ship gates for approval (repeated here so they're not lost):

- Ratio of members who give at least one thumbs-up ≥ some threshold (pick at approval; ~15% for a
  first-month bar).
- The `Most Recommended` sort is used in the catalog non-trivially within a month — otherwise
  members aren't using it as a discovery signal, and the feature isn't earning its keep.

If either fails a month post-launch, consider pruning the feature rather than layering more on top
(e.g. tags) — a signal nobody uses isn't fixed by adding more surface to it.
