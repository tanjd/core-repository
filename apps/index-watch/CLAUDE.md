# CLAUDE.md

Guidance for `apps/index-watch` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, Python/uv tooling, deployment, release process).

Telegram bot (index drawdown tracker), migrated from the standalone `tanjd/index-watch` repo
(squash-imported, no preserved history). Same `uv` + `nx:run-commands` pattern as the
`telegram-bot` generator's own output, just not generator-scaffolded itself (setuptools
build-backend, not hatchling, since that's what the source repo used).

Ruff's `src` setting (which does not propagate through the shared root `ruff.toml`'s `extend`)
is `["src", "tests"]` here, matching the generator template's package layout.

`CHANGELOG.md` was seeded with the standalone repo's pre-migration history (it used
`python-semantic-release`, tag format `v{version}` — an unrelated tool to this repo's
`nx release`) so changelog continuity survives the migration; `nx release` prepends new entries
above it. Nothing in this app reads `CHANGELOG.md`/`package.json` at runtime (contrast
`apps/table-talks`, which does).

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — already bootstrapped; every push to `main` versions it automatically.
