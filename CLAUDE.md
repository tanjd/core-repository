# core-repository

Nx monorepo (TS/Next.js + Go + Python/uv), managed with pnpm, developed inside
a devcontainer. Long-term home for side projects, starting with Telegram bots.

## Workspace layout

- `apps/food-maps` — Next.js frontend.
- `apps/food-maps-backend` — Go API (huma + chi + SQLite via `mattn/go-sqlite3`, cgo).
- `apps/food-maps-e2e` — Playwright E2E tests for `food-maps`.
- `libs/food-maps-data` — shared TS lib, consumed via the `@tanjd/food-maps-data`
  path alias in `tsconfig.base.json` (not an npm package — no `package.json` of
  its own, folded into the root pnpm workspace).
- `tools/generators/telegram-bot` — local Nx generator for scaffolding new bots.

Telegram bots (`index-watch`, `table-talks`, `otobr-buddy`) currently live in
their own standalone repos. Migrating them into this monorepo is a deliberate,
one-at-a-time, separate effort — do not fold a live bot in as a drive-by change.

## Common commands

Prefer the `make` targets below over typing the raw `pnpm nx` invocations —
run `make help` for the full list.

```bash
make setup                              # pnpm install --frozen-lockfile
make verify                             # nx run-many -t build lint test (full local check)
make affected                           # nx affected -t lint test (what CI actually runs)
make nx-reset                           # pnpm nx reset, when a cached result looks stale
make new-bot NAME=foo                   # scaffold a new bot (see "Scaffolding" below)
make docker-build APP=food-maps-backend # docker build -f apps/<app>/Dockerfile .
make golangci-verify                    # confirm .golangci.yaml is actually being loaded
```

Lower-level, for anything not covered above:

```bash
pnpm nx show projects                 # list all projects
pnpm nx <target> <project>            # e.g. pnpm nx test food-maps-backend
```

Husky's pre-commit hook only runs `nx format:write` + generic pre-commit-hooks
checks (whitespace, merge conflicts, etc.) — it's intentionally fast. Lint and
test run once, in CI, via `.github/workflows/ci.yml`'s `nx affected -t lint test`
(triggered on `push`/`pull_request` to `main`). Don't add lint/test back into
the pre-commit hook; that was deliberately removed to avoid double-running the
same work with no cache to offset it (`neverConnectToCloud: true` in `nx.json`).

## Go (`food-maps-backend`)

- Lint config is `.golangci.yaml` at the repo root — note the full "golangci",
  not "golanci". The installed `golangci-lint` is v2, which requires the v2
  config schema (`version: "2"`, linter settings under `linters.settings`, not
  bare top-level keys). Verify a config change is actually being picked up with
  `make golangci-verify` (or `golangci-lint run -v`, looking for
  `[config_reader] Used config file ...`) — a silent fallback to defaults is
  exactly the bug that motivated this note.
- `nx run food-maps-backend:lint` depends on `golangci-lint` (see
  `nx.json` → `targetDefaults.lint.dependsOn`), so a plain `nx affected -t lint`
  genuinely gates on it, not just `go vet`/`go fmt`.

## Scaffolding a new Telegram bot

```bash
make new-bot NAME=<bot-name>
```

Which wraps (and is easier to remember than):

```bash
pnpm nx g ./tools/generators/telegram-bot/generators.json:telegram-bot <bot-name>
```

(The shorter `nx g ./tools/generators/telegram-bot <name>` form doesn't resolve
in this Nx version — local generator collections need either a registered
project or an explicit path to `generators.json`.)

Produces `apps/<bot-name>/` with a `python-telegram-bot` skeleton (+ a
stdlib-only health-check endpoint), `Dockerfile`, `.env.example`, `README.md`,
and a `project.json` with `build`/`serve`/`test`/`lint`/`docker-build` targets
that shell out to `uv` (`nx:run-commands`, same pattern `food-maps-backend`
uses for Go — no language-specific Nx plugin). Uses **uv**, not Poetry, to
match the real standalone bot repos (`index-watch` et al.) and ease their
eventual migration into this monorepo. `@nxlv/python` was removed as a
dependency — nothing here uses it.

The devcontainer's `uv` feature (`ghcr.io/va-h/devcontainers-features/uv`) is
required for the generator's post-generation `uv lock` step and for the
generated targets to run at all.

## Deployment pattern

Any deployable app owns its own `Dockerfile` under `apps/<name>/`, built with
the **repo root as build context**: `make docker-build APP=<name>` (or
`docker build -f apps/<name>/Dockerfile .` — see the root `.dockerignore`,
since `node_modules` alone is 1.3G).

On push to `main`, `.github/workflows/release.yml`:

1. Uses `nx show projects --affected --type app` to find changed apps.
2. Filters to apps that own a `Dockerfile` (not every app is deployable —
   e.g. `food-maps` isn't Dockerized, `food-maps-backend` is).
3. Builds and pushes each to `ghcr.io/tanjd/<app-name>`, tagged `latest` and
   by commit SHA.

GHCR was chosen over Docker Hub (which the standalone bots currently use) to
reuse the `GITHUB_TOKEN` auth already set up in `ci.yml` — not a hard lock-in.

## Known gaps / deferred work

- Nx is capped at 22.2.2, not latest: @nx-go/nx-go (food-maps-backend's Go
  plugin) has zero versions supporting Nx 23+ as of this writing (confirmed
  by `nx show projects` crashing entirely, not just Go targets, under 23.1.0).
  Before running `make upgrade-nx` (or `nx migrate latest`) again, check
  `npm view @nx-go/nx-go dependencies` for its `@nx/devkit` range first.
- food-maps' Next.js build/serve targets pin `"webpack": true` in
  `project.json` — Turbopack (Next 16's default) can't build this workspace
  because it hard-fails on @nx/devkit's optional Angular-schematics adapter
  requires, where webpack just skips them gracefully. Revisit once `@nx/next`
  catches up.
- GitHub branch protection on `main` (requiring the CI check before merge)
  hasn't been enabled — it's a repo-settings change, not a code change.
- pnpm is pinned to the 9.x line (EOL upstream) to match the existing
  lockfile; bumping to 10/11 needs a lockfile migration.
- Migrating `index-watch`/`table-talks`/`otobr-buddy` into this monorepo:
  one at a time, starting with the simplest, only after the generator +
  deploy pattern above have been proven with a throwaway app (they have real
  users/schedules — don't touch them as a drive-by change).
