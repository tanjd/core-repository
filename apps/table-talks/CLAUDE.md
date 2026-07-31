# CLAUDE.md

Guidance for `apps/table-talks` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, Python/uv tooling, deployment, release process).

Telegram bot (theme-based conversation card game), migrated from the standalone
`tanjd/table-talks` repo (squash-imported, no preserved history). Same non-generator-scaffolded
pattern as `apps/index-watch`, with two adaptations beyond the standard cross-cutting drops:

- Its `src/` package was renamed from the standalone repo's flat `src/` (`python -m src.index`)
  to `src/table_talks/` (`python -m table_talks.index`) to match `index-watch`'s and the
  generator's nested-package convention.
- Its bot-info-screen feature (`src/table_talks/version.py`, reads a live version + recent
  changelog entries for in-chat display) was adapted to read `package.json` (the file
  `nx release` actually keeps current) instead of `pyproject.toml`, and to match `nx release`'s
  changelog header format (`#` for the newest entry, `##` for older ones) — `index-watch` has no
  equivalent runtime feature, so this gap was never hit there. `CHANGELOG.md` was seeded with the
  standalone repo's pre-migration `python-semantic-release` history for the same continuity
  reason as `index-watch`, but here it actually matters at runtime because `version.py` reads it.

Ruff's `src` setting (which does not propagate through the shared root `ruff.toml`'s `extend`)
is `["src", "tests"]` here, matching the generator template's package layout.

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — already bootstrapped; every push to `main` versions it automatically.
