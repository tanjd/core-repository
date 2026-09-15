# CLAUDE.md

Guidance for `apps/food-maps` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Next.js frontend. Depends on `libs/food-maps-data` via the `@tanjd/food-maps-data` path alias in
`tsconfig.base.json` — picked up automatically by `@nx/js`'s `analyzeSourceFiles: true`, no
explicit `implicitDependencies` needed (contrast `apps/food-maps-e2e`, which does need one since
Playwright drives this app over HTTP rather than importing it).

Not Dockerized, and not in `nx.json`'s `release.projects` — nothing here is deployed by this
repo's own CI (or anywhere else) yet.

## Known gaps

- Build/serve targets pin `"webpack": true` in `project.json` — Turbopack (Next 16's default)
  can't build this workspace because it hard-fails on `@nx/devkit`'s optional
  Angular-schematics-adapter requires, where webpack just skips them gracefully. Revisit once
  `@nx/next` catches up.

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
