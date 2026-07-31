# CLAUDE.md

Guidance for `apps/ledger-lens-backend` specifically — see the repo-root `CLAUDE.md` for
cross-cutting conventions (Nx, Python/uv tooling, deployment, release process).

Python/uv + FastAPI + SQLModel API (portfolio CSV ingestion + analysis), migrated from the
standalone `tanjd/ledger-lens` repo's `backend/` directory (squash-imported, no preserved
history). Flattened directly into `apps/ledger-lens-backend` rather than nested under a
`backend/` subdir, which shifted `app/config.py`'s default `data_dir` and `app/main.py`'s
version-read path up one directory level — both adapted accordingly. `app/main.py`'s
`/api/version` used to read `importlib.metadata.version(...)` (sourced from `pyproject.toml`);
adapted to read `package.json` directly (the file `nx release` actually keeps current) since
`apps/ledger-lens`'s frontend sidebar version display depends on this staying accurate, same
class of fix as `table-talks`'s `version.py`.

Ruff's `src` setting (which does not propagate through the shared root `ruff.toml`'s `extend`) is
`["app", "tests"]` here, not `["src", "tests"]` — this app's package dir is `app/`, not `src/`.

Dockerized (GHCR) and versioned independently via `nx release` (`release.projects` in root
`nx.json`) — its first-release bootstrap (paired with `apps/ledger-lens`, both pinned to the
version already live on Docker Hub under the standalone repo for continuity) has already run;
every push to `main` versions it automatically now, same as every other `release.projects` entry.
