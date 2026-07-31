# CLAUDE.md

Guidance for `apps/ledger-lens` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Next.js + Tailwind CSS + shadcn/ui frontend for `apps/ledger-lens-backend`, migrated from the
standalone repo's `frontend/` directory (same squash-import, flattened the same way).

Its `Dockerfile` can't reuse the source repo's `npm ci`-based build — this app's `package.json`
is a bare version manifest like every other TS app here (no per-app dependencies; see the
repo-root `CLAUDE.md`'s "Release versioning" section), so the image build runs `pnpm install` at
the repo root and builds via `pnpm exec nx build ledger-lens` instead of a package.json `build`
script. This established the sub-pattern the first three dockerized apps (which have real
per-app dependency manifests — Go modules, `pyproject.toml`) didn't need.

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — its first-release bootstrap (paired with `apps/ledger-lens-backend`, both pinned to
the version already live on Docker Hub under the standalone repo for continuity) has already run;
every push to `main` versions it automatically now, same as every other `release.projects` entry.
