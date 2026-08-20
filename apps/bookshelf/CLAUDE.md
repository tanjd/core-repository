# CLAUDE.md

Guidance for `apps/bookshelf` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Next.js + Tailwind CSS + shadcn/ui frontend for `apps/bookshelf-backend`, squash-imported from the
standalone `tanjd/bookshelf` repo's `frontend/` directory, no preserved history — same convention
as every other app migration in this repo. All calls go through `src/app/api/[...path]/route.ts`,
a Next.js proxy that forwards to `BACKEND_URL` (read at request time, not baked into the build) so
the same image works across environments without a rebuild.

See `apps/bookshelf-e2e/CLAUDE.md` for this app's e2e conventions — including the standing
instruction to verify UI changes through that Playwright suite rather than an ad hoc
headless-browser script.

Its `Dockerfile` follows `apps/ledger-lens`'s sub-pattern rather than a plain `npm ci`/`npm run
build`: this app's `package.json` is a bare version manifest like every other TS app here (no
per-app dependencies; see the repo-root `CLAUDE.md`'s "Release versioning" section), so the image
build runs `pnpm install` at the repo root and builds via `pnpm exec nx build bookshelf` instead.
The source repo had no `public/` directory (favicon is served from `src/app/favicon.ico` via the
Next.js app-router convention) — `@nx/next:build` requires one to exist regardless, so an empty
`public/.gitkeep` was added, matching `food-maps`'s convention.

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — this is its first release; version starts at `0.1.0` (the source repo's own version
history wasn't preserved either).

`next.config.ts` was rewritten to wrap the config in `@nx/next`'s `withNx`/`composePlugins` (the
source repo used a plain `NextConfig` export) — without it, `@nx/next:build` silently doesn't emit
`.next/standalone` under `dist/apps/bookshelf`, which broke the Docker build's `COPY` steps. Same
`nx: {}` + `composePlugins(withNx)` shape as `apps/ledger-lens/next.config.ts` and
`apps/food-maps/next.config.mjs`; `images.remotePatterns` (Open Library / Google Books cover
hosts) and `output: "standalone"` were preserved from the source repo's config.

## Frontend design

See the source repo's original `CLAUDE.md` (not carried over verbatim — summarized here) for the
page-layout system, search-as-hero pattern, and status-color conventions this app's components
follow: full `max-w-6xl` for grid/table pages, `max-w-2xl`/`max-w-lg mx-auto` for narrow
single-column pages, `max-w-md mx-auto` for forms, always pairing a `max-w-*` constraint with
`mx-auto`. `success`/`destructive`/`secondary`/`outline` badge variants map to
available/loaned/pending/terminal states — every status enum in the app (loan requests,
`WaitlistEntry`, `WishlistRequest`'s `open`/`fulfilled`/`cancelled` in `src/lib/wishlist.ts`)
reuses this same four-variant vocabulary rather than introducing new badge colors.

## Mobile-first UI

This app is used from the phone as much as (probably more than) the desktop — community members
requesting/lending books day-to-day — so mobile is a first-class layout target, not a shrink-down
of the desktop view. Concretely:

- **Bottom tab bar, not a hamburger menu** (`src/components/layout/BottomTabBar.tsx`,
  `md:hidden`, fixed to the viewport bottom): primary nav lives in thumb reach instead of behind
  an extra tap. `src/components/layout/navItems.ts`'s `primaryNavItems` is the single source of
  truth for both this and the desktop `NavBar` — add/rename/remove a destination there once.
  - The tab bar is a **fixed 5-slot grid** (`grid-cols-5`): Notifications and Profile/Admin
    always occupy two of the five, so at most **three** `primaryNavItems` entries can have
    `mobileTab: true` (the default) at once. A `NavItem` opts out with `mobileTab: false` when
    the bar is already full — its destination still shows in the desktop `NavBar`, and on mobile
    it's reachable another way (see the FAB pattern below). `Share a Book` and `Wishlist` are
    both `mobileTab: false` today, leaving Catalog/My Books/My Requests as the three tab slots.
    Adding a fourth mobile tab means either bumping every other item's `mobileTab` decision or
    changing the grid — it doesn't just slot in.
  - Padded with `style={{ paddingBottom: "env(safe-area-inset-bottom)" }}` for the iOS home
    indicator; every fixed-to-bottom mobile element in this app (tab bar, FABs) does the same.
- **Content clears the tab bar**: `src/app/layout.tsx`'s `<main>` and `<footer>` both carry
  `pb-24 md:pb-6` — the extra bottom padding only applies below the `md` breakpoint, since the
  fixed tab bar would otherwise cover the last ~6rem of scrollable content on mobile.
- **FAB for actions that lost their tab slot**: a page needing quick access to a `mobileTab:
false` destination adds a `md:hidden fixed` circular button, positioned above the tab bar with
  `bottom: "calc(env(safe-area-inset-bottom) + 4.5rem)"` (safe-area inset plus the tab bar's own
  height). `src/app/catalog/page.tsx` uses a `Popover`-based speed-dial FAB (`Plus` icon, rotates
  45° via `data-[state=open]:rotate-45` when open) once it needed to reach two destinations
  (`/share` and `/wishlist`) from one button — a plain `<Link>` FAB only works for a single
  target.
- **Cards over dense tables on narrow screens**: compact "glance" cards (e.g.
  `CurrentlyBorrowedCard.tsx`, `WishlistCard` in `src/app/wishlist/page.tsx`) show cover + title +
  the one or two facts that matter, in a `grid-cols-1 sm:grid-cols-2 lg:grid-cols-3` grid — full
  detail (message threads, contact info) stays behind an explicit expand/dialog rather than being
  crammed into the card, so the default view stays scannable one-handed.
- Touch targets follow shadcn/ui defaults (`size-icon`/`size-5`+ icons, `py-2`+ tap areas) — avoid
  shrinking interactive elements below those defaults for density on mobile-facing pages.

When adding a new page or primary action, check both the `md:hidden`/`hidden md:*` breakpoint
split and the tab-bar slot budget above before assuming there's room for one more nav entry.

## Environment

`BACKEND_URL` (server-side only, read by the API proxy route) belongs in `apps/bookshelf/.env.local`
for local dev — gitignored via a `apps/bookshelf/.env.local` entry in the root `.gitignore` (no
existing app used `.env.local` before this migration, so the entry is scoped to this app rather
than a workspace-wide glob).
