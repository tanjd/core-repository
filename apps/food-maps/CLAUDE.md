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
