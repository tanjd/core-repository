# CLAUDE.md

This file provides guidance to Claude Code when working with code in `apps/jeddy-tan`. See the
repo-root `CLAUDE.md` for cross-cutting conventions (Nx, monorepo structure, deployment) — this
file stays scoped to jeddy-tan specifics.

## Commands

```bash
pnpm nx serve jeddy-tan     # Start dev server
pnpm nx build jeddy-tan     # Production build (outputs to dist/apps/jeddy-tan)
pnpm nx test jeddy-tan      # Run tests (Vitest)
pnpm nx lint jeddy-tan      # Check lint errors
```

Pre-commit hooks (the shared repo-root Husky + lint-staged setup) run ESLint and `oxfmt` on
staged files automatically — there's no jeddy-tan-local formatting/hook config.

## Architecture

Single-page React portfolio app with client-side routing:

- `src/App.jsx` — Root with React Router routes (`/`, `/projects`, `/experience`) and the shared
  `Navbar` / `Footer`
- `src/pages/` — One component per route (Home, Projects, Experience)
- `src/components/` — Shared UI: `Timeline.jsx`, `Navbar.jsx`, `Footer.jsx`
- `src/data.json` — Single source of truth for all experience entries
- `src/styles/` — Per-component CSS files; responsive breakpoints at 600px and 900px

## Adding or editing experience entries

All data lives in `src/data.json`. Each entry follows this structure:

```json
{
  "date": "May 2021 - Aug 2021",
  "type": "work",
  "title": "Job Title - Company Name",
  "subtitle": "Location (optional)",
  "sections": [
    {
      "heading": "Team or project name",
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
- `sections` / `bullets`: plain text — no HTML needed
- Display order is automatic: active entries first, then sorted by start date (newest first)

## Key patterns

- `Timeline.jsx` consumes `data.json` and renders `react-vertical-timeline-component`. It is used
  by both the Home page and the Experience page.
- Material-UI (`@mui/material`, `@mui/icons-material`) is used for icons; emotion handles
  CSS-in-JS.
- Color palette: dark navy `#192428` backgrounds, `#f0f0f0` text, cyan `#39ace7` accents, gold
  `#d8ab4e` for the active timeline entry.

## Notes from the pre-migration standalone repo

- This app is plain JavaScript (`.jsx`, no TypeScript) — the only JS-only app in this monorepo.
  It's the first `@nx/vite` app here too; every other frontend app uses Next.js via `@nx/next`.
- The Create React App → Vite migration mentioned in older notes for this app is complete; this
  `vite.config.js` is the result.
- Deployed via Cloudflare Pages (outside this repo's GitHub Actions) — see the root `CLAUDE.md`
  for the monorepo build configuration Cloudflare Pages needs.
