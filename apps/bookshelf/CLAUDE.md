# CLAUDE.md

Guidance for `apps/bookshelf` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Next.js + Tailwind CSS + shadcn/ui frontend for `apps/bookshelf-backend`, squash-imported from the
standalone `tanjd/bookshelf` repo's `frontend/` directory, no preserved history — same convention
as every other app migration in this repo. All calls go through `src/app/api/[...path]/route.ts`,
a Next.js proxy that forwards to `BACKEND_URL` (read at request time, not baked into the build) so
the same image works across environments without a rebuild.

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

## Known gaps

- `eslint.config.mjs` downgrades `react-hooks/set-state-in-effect` to a warning. The source repo
  pinned `eslint-config-next` 16.1.6; this workspace's shared root `next`/`eslint-config-next` is
  16.2.12, whose bundled `eslint-plugin-react-hooks` added that rule as an error. It fires on 11
  ported call sites — all "hydrate auth/fetched state on mount" patterns (e.g.
  `src/components/auth/AdminGuard.tsx`, `src/components/auth/SetupGuard.tsx`,
  `src/components/layout/NavBar.tsx`, most of `src/app/**/page.tsx`) — safe as written but flagged
  as an anti-pattern by the newer rule. Rewriting 11 components' effect logic was out of scope for
  a migration pass; consider addressing them (derive state during render, or gate with a ref)
  and dropping the override as a follow-up.

## Frontend design

See the source repo's original `CLAUDE.md` (not carried over verbatim — summarized here) for the
page-layout system, search-as-hero pattern, and status-color conventions this app's components
follow: full `max-w-6xl` for grid/table pages, `max-w-2xl`/`max-w-lg mx-auto` for narrow
single-column pages, `max-w-md mx-auto` for forms, always pairing a `max-w-*` constraint with
`mx-auto`. `success`/`destructive`/`secondary`/`outline` badge variants map to
available/loaned/pending/terminal states respectively.

## Environment

`BACKEND_URL` (server-side only, read by the API proxy route) belongs in `apps/bookshelf/.env.local`
for local dev — gitignored via a `apps/bookshelf/.env.local` entry in the root `.gitignore` (no
existing app used `.env.local` before this migration, so the entry is scoped to this app rather
than a workspace-wide glob).
