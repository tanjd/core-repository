# CLAUDE.md

Guidance for `apps/bookshelf` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

**Documentation:** `README.md` in this directory is the user-facing product guide (self-hosting,
env vars, upgrades). This file is contributor/agent guidance — implementation conventions and
gotchas only; don't duplicate README content here.

Next.js + Tailwind CSS + shadcn/ui frontend for `apps/bookshelf-backend`, squash-imported from the
standalone `tanjd/bookshelf` repo's `frontend/` directory, no preserved history — same convention
as every other app migration in this repo. All calls go through `src/app/api/[...path]/route.ts`,
a Next.js proxy that forwards to `BACKEND_URL` (read from `process.env` at server startup, not
baked into the build) so the same image works across environments without a rebuild.

See `apps/bookshelf-e2e/CLAUDE.md` for this app's e2e conventions — including the standing
instruction to verify UI changes through that Playwright suite rather than an ad hoc
headless-browser script.

See `apps/bookshelf-backend/CLAUDE.md`'s "Product scope" section before proposing metadata-heavy
features (richer book detail pages, ratings/reviews, "more like this") — the product's focus is
identifying books available in the community and facilitating the exchange, not book discovery;
link out to a site like Google Books instead of building that surface here. Ratings, reviews, and
long-form book criticism remain out of scope this way — but a simple member "highly recommend
this" thumbs-up (see `docs/book-recommendations-spec.md`) is in scope, because it surfaces
community reading behaviour adjacent to the exchange flow without implying a rating average or
duplicating an external site's job.

Its `Dockerfile` follows `apps/ledger-lens`'s sub-pattern rather than a plain `npm ci`/`npm run
build`: this app's `package.json` is a bare version manifest like every other TS app here (no
per-app dependencies; see the repo-root `CLAUDE.md`'s "Release versioning" section), so the image
build runs `pnpm install` at the repo root and builds via `pnpm exec nx build bookshelf` instead.
The source repo had no `public/` directory (favicon is served from `src/app/favicon.ico` via the
Next.js app-router convention) — `@nx/next:build` requires one to exist regardless, so an empty
`public/.gitkeep` was added, matching `food-maps`'s convention.

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`); numbering restarted from `0.1.0` at migration rather than continuing the source repo's
own version history (that wasn't preserved either) — current version is whatever
`apps/bookshelf/CHANGELOG.md`'s latest entry says, not necessarily `0.x`.

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
  - The tab bar is a **fixed 5-slot grid** (`grid-cols-5`), Instagram-style: four `Link` tabs
    (Catalog, My Books, Loans, Wishlist — the `primaryNavItems` entries with `mobileTab:
true`, the default) plus a centered, raised **"Share"** button between the first two and last
    two. "Share" is a `Popover` (not a `Link`) offering "Scan ISBN" (`/share/scan`) and "Search"
    (`/share`) — both are ways to add a book, so they share one entry point rather than each
    claiming a tab slot. It's inserted via `tabs.slice(0, Math.ceil(tabs.length / 2))` /
    `tabs.slice(...)` so it stays centered if the tab count changes. `Share a Book` is
    `mobileTab: false` (its destination is reachable via Share's "Search" option instead of its
    own tab). Notifications and Profile do **not** live in this bar — see the mobile header
    below — so all five grid slots go to browse/action destinations. Changing the slot count
    means revisiting this split-and-center logic, not just appending an item.
  - Padded with `style={{ paddingBottom: "env(safe-area-inset-bottom)" }}` for the iOS home
    indicator; every fixed-to-bottom mobile element in this app (tab bar, FABs) does the same.
- **Profile menu, top-right (Facebook-style)**: `src/components/layout/NavBar.tsx` defines a
  shared `ProfileMenu` component (a `Popover` wrapping a Profile/Admin link + Logout row) used
  by both headers, so Logout is never its own always-visible control — tapping the profile
  trigger opens the menu instead. Mobile's `md:hidden` block renders `NotificationBell`
  (unread-badge popover), `ThemeToggle`, then `ProfileMenu` (icon-only trigger,
  rightmost/corner-most). Desktop's `hidden md:flex` block keeps the same consolidation but with
  a text trigger (`{profileItem.label}` + a `ChevronDown`) — desktop still shows the primary nav
  as separate text links and the bell as its own icon (it has the horizontal room for those),
  only Profile+Logout are consolidated to match mobile's interaction pattern.
- **Content clears the tab bar**: `src/app/layout.tsx`'s `<main>` and `<footer>` both carry
  `pb-24 md:pb-6` — the extra bottom padding only applies below the `md` breakpoint, since the
  fixed tab bar would otherwise cover the last ~6rem of scrollable content on mobile.
- **In-bar popover trigger for multi-destination actions**: when one button needs to reach more
  than one destination (like "Share" above), use a `Popover`-based trigger (`Plus`/relevant icon,
  rotates 45° via `data-[state=open]:rotate-45` when open) rather than a plain `<Link>`, which
  only works for a single target. This replaced an earlier per-page FAB on `src/app/catalog/page.tsx`
  (`Share a Book` / `Scan ISBN` / `Wishlist`) once all three destinations got permanent global
  homes (Add popover, Add popover, Wishlist tab) — prefer promoting a frequently-needed action
  into the tab bar/header over adding a new page-local FAB, if there's a sensible slot for it.
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

`BACKEND_URL` (server-side only, read by the API proxy route) belongs in
`apps/bookshelf/.env.local` for local dev — copy from `.env.example`
(`cp .env.example .env.local`). `.env.local` is gitignored via a scoped entry in the root
`.gitignore` (no existing app used `.env.local` before this migration, so the entry is scoped to
this app rather than a workspace-wide glob). Docker/self-host env vars live in
`apps/bookshelf-backend/.env.compose.example` — see `README.md`.

## Changelog and upgrade notices

Release notes for self-hosters and members live at `/changelog`, driven by
`apps/bookshelf/CHANGELOG.md`. At build time, `scripts/generate-changelog.ts` parses that file
into gitignored `src/lib/changelog.generated.ts` (Nx target `generate-changelog`; `build`/`test`/
`lint` depend on it). `useUpgradeNotice` compares `NEXT_PUBLIC_VERSION` against
`localStorage.bookshelf_last_seen_app_version` to surface a dismissible banner in the notification
panel.

The app changelog is written by `nx release` on green CI merges to `main`, not by hand. When a
release includes new backend SQL migrations, a `### Database migrations` subsection is injected
automatically — see `apps/bookshelf-backend/CLAUDE.md` § Release notes and migrations and
`tools/bookshelf-changelog/`. `/changelog` also fetches live `schema_version` from `/api/health`
(proxies backend `/health`) as a footnote for logged-in users.

Spec: `docs/upgrade-changelog-spec.md`.
