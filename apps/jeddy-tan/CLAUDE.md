# CLAUDE.md

This file provides guidance to Claude Code when working with code in `apps/jeddy-tan`. See the
repo-root `CLAUDE.md` for cross-cutting conventions (Nx, monorepo structure, deployment) — this
file stays scoped to jeddy-tan specifics.

Pre-commit hooks (the shared repo-root Husky + lint-staged setup) run ESLint and `oxfmt` on
staged files automatically — there's no jeddy-tan-local formatting/hook config.

## Architecture

Single-page React portfolio app with client-side routing:

- `src/App.jsx` — Root with React Router routes (`/`, `/projects`, `/experience`) and the shared
  `Navbar` / `Footer` (Navbar is rendered on every route)
- `src/pages/` — One component per route (`Readme.jsx` at `/`, `Projects.jsx`, `Experience.jsx`)
- `src/components/` — Shared UI: `Timeline.jsx`, `TimelineEntryContent.jsx`, `Navbar.jsx`,
  `Footer.jsx`
- `src/data.json` — Single source of truth for all experience entries
- `src/projectsData.json` — Side/hobby projects shown on the Projects page (this monorepo,
  Telegram bots, self-hosted NAS, OSS contributions — distinct from work experience)
- `src/styles/` — Per-component CSS files; responsive breakpoints at 600px and 900px

## Adding or editing experience entries

All data lives in `src/data.json`. Each entry follows this structure:

```json
{
  "date": "May 2021 - Aug 2021",
  "type": "work",
  "title": "Job Title - Company Name",
  "subtitle": "Location (optional)",
  "highlights": [
    "Punchy, always-visible summary bullet",
    "Another one — 2-4 total is a good range"
  ],
  "sections": [
    {
      "heading": "Team or project name",
      "eli5": "Optional plain-language analogy for this one project/team.",
      "bullets": [
        "What you did, written in plain English.",
        "Another bullet point."
      ]
    }
  ],
  "isActive": true,
  "order": 1
}
```

- `type`: `"work"` or `"education"`
- `isActive`: `true` highlights the entry in gold and pins it to the top (use for current role)
- `highlights` (optional, top-level, array of strings): always-visible summary shown on every
  timeline entry. If omitted, `TimelineEntryContent.jsx` falls back to the entry's first bullet —
  not every entry has `highlights` filled in yet (currently only Autodesk and ExpressVPN do; CSA,
  GovTech, and SMU rely on the fallback).
- `sections[].eli5` (optional string): a plain-language analogy for that one section/project,
  toggled in place of that section's bullets via the lightbulb icon button. Optional per section —
  most sections don't have one yet; only fill in where an analogy genuinely helps.
- `sections` / `bullets`: plain text — no HTML needed
- Display order is automatic: active entries first, then sorted by start date (newest first)

## Adding or editing side projects

Side/hobby projects (as opposed to work experience) live in `src/projectsData.json`, rendered as
cards on the Projects page. Each entry:

```json
{
  "name": "Project name",
  "description": "One or two sentences, plain English.",
  "eli5": "Optional plain-language analogy.",
  "tags": ["Tech", "or", "category", "tags"],
  "links": { "github": "https://...", "live": "https://..." }
}
```

`eli5`, `tags`, and both `links` sub-fields are optional.

## Key patterns

- `Timeline.jsx` consumes `data.json` and renders `react-vertical-timeline-component` directly (no
  wrapper div of its own — the page component supplies that); the actual per-entry content (title,
  highlights, expand/collapse, ELI5 toggles) is delegated to `TimelineEntryContent.jsx` so it can
  hold that interactive state. `Timeline` accepts a `variant` prop: `"full"` (default, the only
  variant actually in use — Experience is Timeline's sole remaining consumer) adds a "Show more"
  toggle revealing `sections`/`bullets`, plus a per-section lightbulb button that swaps bullets for
  that section's `eli5` text when present. `"condensed"` (highlights only, no expand/ELI5 UI) was
  built for the old standalone Home page's teaser and has had no consumer since Home was folded
  into `Readme.jsx` — dead code kept around in case a condensed teaser resurfaces elsewhere, not a
  bug.
- Material-UI (`@mui/material`, `@mui/icons-material`) is used for icons, `Collapse`/`Button`/
  `IconButton` (expand + ELI5 toggles), and `Card`/`Grid`/`Chip` (Projects page); emotion handles
  CSS-in-JS. There's no global MUI theme/`ThemeProvider` — colors are plain CSS classes throughout,
  by design, matching the rest of the app.
- Color palette: dark navy `#192428` backgrounds, `#f0f0f0` text, cyan `#39ace7` accents, gold
  `#d8ab4e` for the active timeline entry. This is a single unified dark theme across every
  route (Home/Readme, Projects, Experience) plus the Navbar/Footer chrome — `.App`'s own background
  (`App.css`) is set to the same dark navy too, since it peeks through in the gap above the Footer
  (`margin-top: 200px` in `Footer.css`) on any route shorter than the viewport; it's not just inert
  fallback color. The Experience/Timeline page reaches this state by overriding
  `react-vertical-timeline-component`'s default light theme (white cards, white icon-ring halo,
  dark line/icon on white bg) in `Timeline.css`, scoped under the page's own `.experience` class
  for specificity — the vertical line color and non-active timeline-icon colors are inverted from
  the library's light-theme defaults (light icon fill on dark bg, muted slate line) rather than
  reused verbatim.
- The README page (`src/pages/Readme.jsx`, route `/`, nav label `$ home`) is a deliberate pun — it
  doubles as the site's landing page (hero greeting + social links, formatted as a fake README
  "title" section) and a place to write about work values/working style, framed like an actual
  README file (monospace path/heading styling in `Readme.css`). It replaced a separate `Home.jsx`
  page that used to render at `/` with its own condensed `Timeline` teaser — that duplicated the
  full `Timeline` already shown on `/experience`, so it was dropped in favor of this page serving
  both roles. Its content (both the hero copy and the `principles` array) is inline JSX/JS at the
  top of the component, **not** a JSON data file like `data.json`/`projectsData.json` — those JSON
  files earn their keep as repeatable, growing datasets; this page is a handful of one-off prose
  entries with a single consumer.
- The site favicon set (`public/favicon.ico`, `logo192.png`, `logo512.png`,
  `apple-touch-icon.png`) was generated from a source avatar image via a one-off Pillow crop/resize
  script (not a build-time dependency — Pillow isn't in any `pyproject.toml`/`requirements.txt`
  here). The uncropped source art lives in `apps/jeddy-tan/assets-src/jeddy.png` — not in `public/`,
  since it's not referenced by the app at runtime and would otherwise ship an unused ~700KB file.

## Content voice

When writing or revising copy anywhere on this site (Home hero, Readme principles, project
descriptions, or any future page), this principle should shape tone and content choices — it
should never appear as an explicit statement on the site itself:

Software engineering is what Jeddy does professionally, not the whole of who he is. What he does
and who he is are related but distinct. Avoid conflating the two in copy — don't lean on
"passionate software engineer" as a totalizing self-description, don't let job titles or tech
stack stand in for personality, and leave room for whatever isn't strictly professional (how he
thinks, what he's curious about, his values) rather than writing this site as a resume with a
personality veneer.

This is a standing guideline, not a one-time edit — it doesn't mandate changing any specific page
today. The placeholder `principles` array in `Readme.jsx` (see "Key patterns" above) is the most
obvious place to apply it next time that page's content is written for real.

## Notes from the pre-migration standalone repo

- This app is plain JavaScript (`.jsx`, no TypeScript) — the only JS-only app in this monorepo.
  It's the first `@nx/vite` app here too; every other frontend app uses Next.js via `@nx/next`.
- The Create React App → Vite migration mentioned in older notes for this app is complete; this
  `vite.config.js` is the result.
- Deployed via Cloudflare Pages (outside this repo's GitHub Actions) — see "Cloudflare Pages
  reconfiguration" below for the monorepo build configuration Cloudflare Pages needs.

## Cloudflare Pages reconfiguration (one-time, pending)

The `jeddy-tan` Cloudflare Pages project still points at the old standalone `tanjd/jeddy-tan`
repo — needs a one-time manual dashboard reconfiguration after this migration is merged to
`main` (do this **last**, only once `apps/jeddy-tan` actually exists on `main` — flipping the
source repo any earlier breaks the next auto-build and takes the live site down). In the
`jeddy-tan` Pages project's Settings → Builds & deployments (this is classic Pages, not the newer
Workers Builds product):

- Re-point the connected Git repo to `tanjd/core-repository`, production branch `main`.
- Framework preset: `None` (a preset like "Vite" prefills a bare `dist` path assuming a
  non-monorepo layout — wrong here).
- Root directory: leave blank/repo root, **not** `apps/jeddy-tan` — dependencies are hoisted to
  the workspace root, same reasoning as `ledger-lens`'s Dockerfile (see
  `apps/ledger-lens/CLAUDE.md`).
- Build command: `pnpm install --frozen-lockfile && pnpm exec nx build jeddy-tan`
- Build output directory: `dist/apps/jeddy-tan`
- Build watch paths (separate section, same page): include `apps/jeddy-tan/**`, `package.json`,
  `pnpm-lock.yaml`, `nx.json`, so unrelated app changes don't trigger a redeploy.
- Verify with a manual retry-deployment before trusting it to auto-build.
