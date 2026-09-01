# Wishlist CTA & search-box clarity — spec

**Status:** Implemented (#96). **Scope:** `apps/bookshelf/src/app/wishlist/page.tsx`
only. No backend/API changes, no changes to `/share`, `MetadataSearchStep`, or
`CreateRequestDialog`'s internals beyond a small prefill prop. **Supersedes (for now):**
`docs/wishlist-search-unification-spec.md`, which is on hold — see that file's "Revision notes" if
this gets revisited later.

## Problem

Recap from `wishlist-search-unification-spec.md`: the `/wishlist` page's own search box (which only
**filters** the existing `WishlistRequest` list) looks visually identical to the search box used to
**add** a book elsewhere in the app — same `Search` icon, near-identical placeholder copy ("Search
by title, author…"). Members type the book they want into the filter box expecting it to add the
book; the actual "add" action is a separate, less prominent `Plus`/"Add to wishlist" button.

This is the smaller-scope fix: make the two boxes stop looking alike, and catch the mistake if it
still happens — without touching `/share`'s search/state machine or `CreateRequestDialog`'s
internals. See `wishlist-search-unification-spec.md` for the deferred larger fix (unifying the
duplicate search implementations across the app).

## Fix — restyle, clarify, bridge

All changes are inside `wishlist/page.tsx`.

1. **Clearer, more prominent "Add to wishlist" CTA.** Restyle the current `Plus`/"Add to wishlist"
   button (lines ~134-137) so it reads as the page's primary action rather than competing visually
   with the filter box below it. Swap copy/icon to match the wording used for this same action
   elsewhere in the app:

   ```tsx
   <Button onClick={() => openCreateDialog()}>
     <Heart className="size-4" />
     Request a Book
   </Button>
   ```

2. **Restyle the filter box so it reads as filter-only.** Icon `Search` → `ListFilter`; placeholder
   "Search by title, author…" → "Filter requests…"; visually secondary/muted relative to the CTA
   (narrower width, smaller height, muted background/border). Same position in the layout, no DOM
   reordering.

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

3. **Empty-state bridge.** When the filter box's query returns zero matches, replace the current
   plain "No matches for your search." text (lines ~159-166) with a bridge into the existing
   add-flow, pre-filled with the typed query — this is what actually catches the mistake at the
   moment it happens, rather than relying on the user separately noticing the restyled CTA.

   ```tsx
   ) : requests.length === 0 ? (
     search.trim() ? (
       <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
         <p className="text-muted-foreground">
           No existing requests match &ldquo;{search.trim()}&rdquo;.
         </p>
         <p className="text-sm text-muted-foreground">Want to request it?</p>
         <Button size="sm" onClick={() => openCreateDialog(search.trim())}>
           <Heart className="size-4" />
           Request &ldquo;{search.trim()}&rdquo;
         </Button>
       </div>
     ) : (
       <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
         <p className="text-muted-foreground">The wishlist is empty right now.</p>
       </div>
     )
   ) : (
   ```

   `CreateRequestDialog` needs one small addition — an `initialQuery?: string` prop, seeding its
   own internal `query` state on open (same idea as `/share/page.tsx`'s existing `?q=` prefill for
   `MetadataSearchStep`, but as a plain prop since this dialog is opened in-page, not navigated
   to):

   ```tsx
   function CreateRequestDialog({
     open,
     onOpenChange,
     onCreated,
     initialQuery = "",
   }: {
     open: boolean;
     onOpenChange: (open: boolean) => void;
     onCreated: () => void;
     initialQuery?: string;
   }) {
     const [query, setQuery] = useState(initialQuery);
     // ...unchanged below
   ```

   `WishlistPage` gains a small wrapper around today's `setCreateOpen` to carry the prefill:

   ```tsx
   const [createQuery, setCreateQuery] = useState("");
   function openCreateDialog(prefill = "") {
     setCreateQuery(prefill);
     setCreateOpen(true);
   }
   ```

   Pass `initialQuery={createQuery}` to `<CreateRequestDialog>`. `CreateRequestDialog`'s own
   `reset()` (called on close) should not touch `createQuery` — it's owned by the parent and is
   only ever set again by the next `openCreateDialog()` call.

## Known gaps (deferred to the larger spec if it's revisited)

- `CreateRequestDialog` keeps its own independently-implemented metadata search box — not
  `MetadataSearchStep` — so it still lacks that component's result-bucketing and "enter manually"
  fallback. Not fixed here; that's the specific problem `wishlist-search-unification-spec.md`
  addresses.
- Catalog's "Add to wishlist" empty-state link (`catalog/page.tsx` line ~390) still points to
  `/wishlist?q=...`, which this page still doesn't read — it remains a dead prefill. A low-cost
  partial fix (read `?q=` on mount, call `openCreateDialog(q)` automatically) is possible if this
  turns out to matter, but is left out here to keep this spec's scope to the wishlist page itself.

## Verification

- Manually verify: the CTA reads as the primary action, the filter box reads as secondary/
  filter-only, typing a title with no matching existing request surfaces the bridge, and the
  bridge opens `CreateRequestDialog` pre-filled with the typed query.
- No existing e2e test covers wishlist request creation at all (confirmed via grep;
  `apps/bookshelf-e2e/CLAUDE.md` already flags this as untested) — not blocking for this scope, but
  worth a minimal spec covering bridge → dialog-prefill → submit if time allows.
- `pnpm nx affected -t lint test e2e` before merging.
