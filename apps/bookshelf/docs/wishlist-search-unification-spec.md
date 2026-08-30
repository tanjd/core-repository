# Wishlist search unification — spec

**Status:** On hold — superseded for now by the smaller-scope `docs/wishlist-cta-clarity-spec.md`
(CTA/box restyle + empty-state bridge, no `/share` changes). Revisit this spec if the
duplicate-search-implementation problem (see "Problem") becomes worth fixing on its own.
**Scope:** `apps/bookshelf` frontend only. No backend/API changes. **Depends on:**
`MetadataSearchStep`, `share/page.tsx`'s step state machine, `wishlist/page.tsx`'s
`CreateRequestDialog`, `catalog/page.tsx`'s empty state. **Not touched:** `/share/scan` — barcode
scanning implies the scanner is holding the book, so it stays a share-only flow by design; no
request branch needed there.

## Revision notes (design review, 2026-08-30)

Before this was put on hold, design review identified one change worth carrying forward if this
spec is revisited: **Fix 1 should not add a mandatory "what do you want to do with this book?"
interstitial.** Every entry point into `/share` already encodes intent via the `?intent=request`
param (or its absence, meaning share). `handleSelectResult` should branch straight to `confirm` or
`request` based on that signal, with a lightweight pivot link on each destination for the rare
mis-route — no new `"intent"` step in the `Step` union. This removes the "one extra click for the
majority case" tradeoff the original draft accepted, and stops it from undermining Fix 3's
empty-state bridge (a user arriving via `intent=request` would otherwise have to re-declare intent
a third time, immediately after already stating it twice). Fix 5's hero-heading change should
likewise become conditional on `suggestedIntent` (default: unchanged "Share a Book" copy) rather
than a blanket rename to intent-neutral copy for every visitor, since the default flow's
destination doesn't change. Nothing in `navItems.ts`/`BottomTabBar.tsx`'s "Share" labeling needs
renaming either, for the same reason — the default (no `intent` param) path stays share-only.

## Problem

User feedback: the `/wishlist` page is confusing, especially on mobile. Its own search box (which
only **filters** the list of existing wishlist requests) looks visually identical to the search box
used to **add** a book elsewhere in the app — same icon, near-identical placeholder copy ("Search
by title, author…" vs "Search by title, author, ISBN…"). Members type the book they want into the
page's filter box expecting it to add the book, when the actual "add" action is a separate `Plus`
button ("Add to wishlist") that opens a dialog containing a _third_, independent search box.

Investigation confirmed the problem is structural, not just cosmetic. `wishlist/page.tsx` contains
three separate search inputs:

1. Page-level filter box (lines ~140-149) — narrows the existing `WishlistRequest` list via
   `api.getWishlistRequests({ q })`.
2. `CreateRequestDialog`'s own metadata search box (lines ~536-546) — hits `api.searchMetadata`
   to find a book to actually create a new request, plus a dedupe check (`api.checkWishlistRequest`)
   offering "I want this too" / "Add it separately anyway".
3. `LinkBookDialog`'s catalog search (lines ~703-713) — admin-only, links a fulfilled request to an
   existing catalog `Book`. **Out of scope for this spec.**

Box 2 is also a second, independently-implemented copy of logic that already exists — with better
UX (result-bucketing, "enter manually" fallback) — in
`apps/bookshelf/src/app/share/components/MetadataSearchStep.tsx`, the component that powers the
`/share` ("Share a Book") flow. Both call the same backend endpoint (`api.searchMetadata`).

## Direction

Rather than re-skinning the wishlist page's boxes in place, eliminate the duplicate search
implementation by unifying "search for a book to add" into one flow shared by both sharing and
requesting — consistent with this app's own documented pattern
(`apps/bookshelf/CLAUDE.md` § "Mobile-first UI") of consolidating multiple "ways to add a book"
into one entry point (the bottom-tab "Share" popover) rather than growing more separate ones.

1. **One search experience app-wide.** `MetadataSearchStep` stays the only "search for a book by
   title/author/ISBN" implementation — no changes to its search/debounce/bucketing logic.
2. **Choose the outcome after finding the book, not before.** A new step in `/share`'s state
   machine, between search and confirm: _"What do you want to do with this book?"_ — "I have a
   copy — share it" (existing confirm flow, unchanged) or "I want to read this — request it" (new
   branch, creates a `WishlistRequest`).
3. **`/wishlist` loses its own add-flow entirely.** `CreateRequestDialog` (button, inline search,
   dedupe check, notes/anonymous form) is deleted. The page's CTA becomes a link into the unified
   flow: `/share?intent=request`.
4. **The wishlist filter box stays, restyled so it stops reading as "search to add."** Icon
   `Search` → `ListFilter`; placeholder "Search by title, author…" → "Filter requests…"; visually
   secondary/muted relative to the new CTA. No DOM reordering.
5. **Empty-state bridge (highest-leverage fix).** When the filter box's query returns zero
   matches, show `No existing requests match "{query}". Want to request it?` with a button routing
   into the unified flow, pre-seeded with that query. This catches the exact mistake being
   reported, at the exact moment it happens.
6. **Same bridge pattern extended to Catalog, empty-results only** (deliberately _not_ a merged
   search — catalog's own search stays fast/local; a true combined local+external search was
   considered and rejected, since it adds external-API latency to routine catalog browsing and
   pushes against the product-scope note in `apps/bookshelf-backend/CLAUDE.md` that this app isn't
   a book-discovery product).
7. **`LinkBookDialog` untouched.**

## Fix 1 — New "intent" step in `/share`'s state machine

**Priority: high — this is the structural core the rest depends on.**

`apps/bookshelf/src/app/share/page.tsx` currently has:

```ts
type Step = "search" | "confirm" | "manual";
```

Extend to:

```ts
type Step = "search" | "intent" | "confirm" | "manual" | "request";
```

`handleSelectResult` (currently ends with `setStep("confirm")`) instead ends with
`setStep("intent")`. `SelectedBook` already carries every field both branches need — no interface
change required.

Add two new mounted-but-`hidden` blocks, following the exact convention already used for
`search`/`confirm`/`manual` (all three stay mounted simultaneously, toggled via
`className={step === "x" ? "" : "hidden"}`, **not** early `return`s — the comment at line ~198
explains this was already fixed once after a real regression where "back to search" silently
discarded `MetadataSearchStep`'s state, and `share-search-back-navigation.spec.ts` enforces it):

```tsx
<div className={step === "intent" && selected ? "" : "hidden"}>
  {selected && (
    <div className="flex flex-col gap-6 max-w-lg mx-auto">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setStep("search")}
        className="self-start -ml-1"
      >
        <ArrowLeft className="size-4" /> Back to search
      </Button>

      {/* Reuse the existing book-preview <Card> markup from the confirm step, ~lines 310-333 */}

      <div>
        <h1 className="text-2xl font-bold">
          What do you want to do with this book?
        </h1>
      </div>

      <div className="flex flex-col gap-3">
        <Button
          size="lg"
          variant={suggestedIntent === "request" ? "outline" : "default"}
          onClick={() => setStep("confirm")}
        >
          <BookPlus className="size-4" />I have a copy — share it
        </Button>
        <Button
          size="lg"
          variant={suggestedIntent === "request" ? "default" : "outline"}
          onClick={() => setStep("request")}
        >
          <Heart className="size-4" />I want to read this — request it
        </Button>
      </div>
    </div>
  )}
</div>
```

Both choices stay clickable regardless of `suggestedIntent` — it only affects which button reads
as visually primary (`default` vs `outline` variant), never which is auto-selected. This is
required: a member who arrives via the wishlist "Request a Book" CTA must still be able to
discover they actually own a copy and pivot to sharing it, and vice versa.

Add a small pivot link to the _existing_ confirm step, next to its current "Not the right edition?
Go back →" link (~line 342-347):

```tsx
<button
  onClick={() => setStep("request")}
  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
>
  Want to request instead of share it? →
</button>
```

This lets someone who lands on confirm directly (a plain `/share` visit, or anyone who chose
"share" on the intent step) still switch branches without re-searching.

### Query params and prefill

Extend the existing `?q=` prefill effect (lines ~60-66) to also read `?intent=request`:

```ts
const [suggestedIntent, setSuggestedIntent] = useState<"request" | null>(null);
// inside the existing prefilledRef effect:
setSuggestedIntent(
  new URLSearchParams(window.location.search).get("intent") === "request"
    ? "request"
    : null,
);
```

| Entry point                           | URL                                                                                         |
| ------------------------------------- | ------------------------------------------------------------------------------------------- |
| Bottom-tab "Share" popover → "Search" | `/share` (unchanged)                                                                        |
| Catalog empty-state "Share this book" | `/share?q=<title>` (unchanged)                                                              |
| Wishlist page "Request a Book" CTA    | `/share?intent=request`                                                                     |
| Wishlist filter empty-state bridge    | `/share?q=<title>&intent=request`                                                           |
| Catalog empty-state "Add to wishlist" | `/share?q=<title>&intent=request` (was `/wishlist?q=...` — see Fix 4, this was a dead link) |

## Fix 2 — New component: `RequestWishlistStep`

New file: `apps/bookshelf/src/app/share/components/RequestWishlistStep.tsx`.

Ports the logic currently inside `CreateRequestDialog` (`wishlist/page.tsx`, current lines
~299-621) essentially verbatim, re-hosted as a page section instead of a `Dialog`:

- Dedupe/match-check effect (`api.checkWishlistRequest`), keyed on the selected book only (not on
  `step`) — fires as soon as a book is selected during the `intent` step, same timing as the
  original dialog, so the result is already resolved by the time the user reaches this step.
- "This book's already on someone's wishlist" match card, with "I want this too" /
  "Add it separately anyway".
- Notes + anonymous-checkbox form, submitting via `api.createWishlistRequest`.
- On success: `toast.success(...)`, then navigate to `/wishlist` (`router.push`) instead of closing
  a dialog.

```tsx
interface RequestWishlistStepProps {
  selected: SelectedBook;
  onBack: () => void; // → setStep("intent")
  onRequested: () => void; // → router.push("/wishlist")
}
```

Files touched:

| File                                       | Change                                                         |
| ------------------------------------------ | -------------------------------------------------------------- |
| `share/components/RequestWishlistStep.tsx` | New — ported logic, described above                            |
| `share/page.tsx`                           | Render `<RequestWishlistStep>` in the new `request`-step block |

## Fix 3 — `wishlist/page.tsx`: remove the old add-flow, restyle the filter box

**Delete:** `CreateRequestDialog` (lines ~299-621), the `SelectedBook` interface (only used
there), `createOpen` state and its dialog render, now-unused imports (`Plus`, `BookMetadataResult`,
`Checkbox`, `Textarea`, `Label` — `LinkBookDialog` doesn't need any of these).

**Keep:** `Dialog`-family imports and the `Search` icon (`LinkBookDialog` still uses both).

**Add:** `Link` from `next/link`, `ListFilter` and `Heart` from `lucide-react`.

New CTA (replaces the `Plus`/"Add to wishlist" button, lines ~134-137):

```tsx
<Link href="/share?intent=request">
  <Button>
    <Heart className="size-4" />
    Request a Book
  </Button>
</Link>
```

Filter box restyle (replaces lines ~140-149):

```tsx
<div className="relative max-w-sm">
  <ListFilter className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
  <Input
    type="search"
    placeholder="Filter requests…"
    value={search}
    onChange={(e) => setSearch(e.target.value)}
    className="pl-9 h-9 bg-muted/30 border-muted-foreground/20"
  />
</div>
```

Narrower width (`max-w-sm` vs the old `max-w-xl`), smaller height, muted background/border — reads
as visually secondary next to the new solid-primary CTA. Same position in the layout; no
DOM reordering (per explicit decision — mobile fix is styling-only, not restructuring).

Empty-state bridge (replaces the `search.trim() ? "No matches for your search." : ...` branch,
lines ~159-166):

```tsx
) : requests.length === 0 ? (
  search.trim() ? (
    <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
      <p className="text-muted-foreground">
        No existing requests match &ldquo;{search.trim()}&rdquo;.
      </p>
      <p className="text-sm text-muted-foreground">Want to request it?</p>
      <Link href={`/share?q=${encodeURIComponent(search.trim())}&intent=request`}>
        <Button size="sm">
          <Heart className="size-4" />
          Request &ldquo;{search.trim()}&rdquo;
        </Button>
      </Link>
    </div>
  ) : (
    <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
      <p className="text-muted-foreground">The wishlist is empty right now.</p>
    </div>
  )
) : (
```

This is the single highest-leverage fix — it catches the exact mistake being reported (typing a
book title into what looks like an "add" box) at the exact moment it happens, regardless of how
well the rest of the redesign lands.

## Fix 4 — `catalog/page.tsx`: fix the dead wishlist prefill link, point both empty-state links at the unified flow

Catalog's empty state already has "Own a copy? Share" (→ `/share?q=...`) and "Add to wishlist"
(→ `/wishlist?q=...`) links. The second is currently a **dead prefill** — `wishlist/page.tsx` never
reads `?q=` from the URL. Fix: repoint it to `/share?q=<query>&intent=request`, consistent with the
new single entry point.

Open implementation-time call: whether to merge these two links into one "Add this book" button
landing on the intent step directly, vs. keeping two separate links each pre-selecting a different
`intent=` value. Either is compatible with this spec; not resolved here.

## Fix 5 — `MetadataSearchStep.tsx`: generalize the hero heading

Its hero heading is hardcoded to "Share a Book." Once every visitor (not just `intent=request`
ones) lands on the new intent-choice step afterward, that heading misleads anyone who'll choose
"request." Generalize to intent-neutral copy, e.g.:

- Heading: "Find a Book"
- Subtitle: "Search by title, author, or ISBN — we'll ask what to do with it next."

Small string change only; confirmed the existing e2e test doesn't assert on this text.

## Known gaps (flagged, not resolved by this spec)

- **No manual-entry path for requests.** `MetadataSearchStep`'s "Can't find your book? Enter
  manually" fallback only leads into the share flow's manual form. A metadata-search miss has no
  equivalent "create a wishlist request manually" path. Proposed: accept the gap for v1 (metadata
  misses are the minority case); revisit if it proves to matter.
- **One extra click for the majority (share) case.** Every plain `/share` visit now goes
  search → intent → confirm instead of search → confirm directly. This is the explicit tradeoff of
  unifying the two flows.

## Build order

1. Fix 1 (intent step + query params) — the structural core; everything else depends on it.
2. Fix 2 (`RequestWishlistStep`) — depends on Fix 1's `request` step existing to render into.
3. Fix 3 (wishlist page changes) — independent of Fix 2's internals, only needs the `/share?intent=request` URL to exist.
4. Fix 4 (catalog link fix) — independent, one-line href change per link.
5. Fix 5 (hero copy) — independent, cosmetic.

## Verification

- Update `apps/bookshelf-e2e/src/share-search-back-navigation.spec.ts`: it currently clicks a
  search result and immediately asserts the "Confirm & share" heading — it will need to land on
  the intent step first, click "I have a copy — share it", then assert as before. The rest of the
  test (query preserved on "Back to search") is unaffected.
- `apps/bookshelf-e2e/src/primary-navigation.spec.ts` is unaffected (only checks the Share
  popover's own links/hrefs, not what's beyond them).
- No existing e2e test covers wishlist request creation at all (confirmed via grep;
  `apps/bookshelf-e2e/CLAUDE.md` already flags this as untested). Add a new spec,
  e.g. `share-request-wishlist-flow.spec.ts`: search → intent step shows both choices → choose
  "request it" → notes/anonymous form → submit → redirected to `/wishlist` with the new request
  visible; plus the dedupe path (two users, same book → "already on someone's wishlist" card).
- Manually verify the wishlist and catalog empty-state bridges navigate to `/share` with correct
  `q`/`intent` params, and that the intent step reflects the suggested choice without blocking the
  other one.
- Full gate before merging: `pnpm nx affected -t lint test e2e`.

## Open decision

This is a bigger change than a page-local visual fix — it touches `/share`'s state machine, adds
two new components, and changes the primary "add a book" flow for every user, not just wishlist
visitors. Before building: confirm this is worth it over a smaller-scope alternative (icon/copy
restyle of the wishlist filter box + empty-state bridge only, leaving `CreateRequestDialog` and its
duplicate search in place). The case for the larger change is that it removes a second
independently-maintained "search for a book" implementation and matches this app's existing
"consolidate multiple add-a-book entry points" precedent (`apps/bookshelf/CLAUDE.md`) — but it's a
real tradeoff against scope/risk that's worth a deliberate yes, not a default.
