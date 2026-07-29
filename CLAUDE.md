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

pnpm is pinned via `.devcontainer/devcontainer.json`'s `pnpmVersion` (no
`packageManager` field in `package.json` — the devcontainer is the single
source of truth for both local dev and CI, since `ci.yml` builds it via
`devcontainers/ci@v0.3`). pnpm 10+ blocks dependency install/postinstall
scripts by default (a security default); packages that need one must be
allow-listed in `package.json` → `pnpm.onlyBuiltDependencies` (currently
just `nx`, whose postinstall does its local setup) or the script silently
no-ops and `pnpm install` prints an "Ignored build scripts" warning.

Lower-level, for anything not covered above:

```bash
pnpm nx show projects                 # list all projects
pnpm nx <target> <project>            # e.g. pnpm nx test food-maps-backend
```

Husky (`core.hooksPath=.husky/_`) is just the trigger Git calls on
`git commit`; `.husky/pre-commit` runs three layers:

- `lint-staged`: ESLint `--fix` + Prettier on staged JS/TS, Prettier alone on
  staged JSON/MD/YAML/CSS/HTML, `gofmt -w` + `goimports -w` on staged Go
  (`goimports` is installed by `make setup`, not part of the Go toolchain by
  default). Staged files only.
- The Python `pre-commit` framework, for the generic `pre-commit-hooks`
  checks (trailing whitespace, EOF newline, YAML/JSON sanity, merge-conflict
  markers, etc.), run against `git diff --cached` (staged), not the working
  tree — fixes from both layers are re-staged so they land in the commit.
- `pnpm nx affected -t lint test` — the full Nx lint/test for whatever the
  commit affects, not staged-file-scoped like the two layers above.

The third layer duplicates what CI runs, on purpose: failing locally before a
push beats failing in CI, even with no remote cache to offset it
(`neverConnectToCloud: true` in `nx.json`) — local `.nx/cache` still speeds up
repeat commits that don't touch a given project. CI (`.github/workflows/ci.yml`,
triggered on `push`/`pull_request` to `main`) still runs the same
`nx affected -t lint test` as the authoritative gate, since `--no-verify` or a
merge can land changes the hook never saw.

## Nx conventions

- **Caching**: `test`, `lint`, `tidy`, and `golangci-lint` are cached via
  `targetDefaults` in `nx.json`. `neverConnectToCloud: true` keeps caching
  local-only, no remote/distributed cache (there's no `nxCloudId` to connect
  with anyway) — local cache still speeds up repeat `nx affected` runs (both
  pre-commit and CI) when a project's inputs haven't changed.
- `namedInputs.sharedGlobals` includes `.github/workflows/ci.yml` — editing
  that file busts the cache for every project.
- **Inferred tasks**: `@nx/next`, `@nx/playwright`, `@nx/eslint`, `@nx/jest`,
  and `@nx-go/nx-go` are registered Nx plugins (`nx.json` → `plugins`) that
  auto-register targets from config files already present in a project
  (`next.config.js`, `playwright.config.ts`, `eslint.config.mjs`,
  `jest.config.ts`) — no need to hand-write those targets for a new TS
  app/lib. Go and Python have no such plugin coverage for their language-
  specific targets, so those are hand-defined `nx:run-commands` targets in
  `project.json` instead (see the Go and Python sections below).
- **Cross-project dependencies** are declared two ways:
  - Inferred automatically from TS imports, via `@nx/js`'s
    `analyzeSourceFiles: true` (`nx.json` → `pluginsConfig`) — e.g.
    `food-maps` → `libs/food-maps-data` through the `@tanjd/food-maps-data`
    path alias, with no explicit config.
  - Declared explicitly via `implicitDependencies` in `project.json` when
    there's no source-level import to analyze — e.g.
    `apps/food-maps-e2e/project.json` sets
    `"implicitDependencies": ["food-maps"]` since Playwright drives the app
    over HTTP rather than importing it.
  - `pnpm nx graph` shows the resulting dependency graph.
- **Module boundaries**: `@nx/enforce-module-boundaries` (in
  `eslint.config.mjs`) is wired up but currently wide open — a single
  `{ sourceTag: "*", onlyDependOnLibsWithTags: ["*"] }` constraint, and every
  `project.json` has `"tags": []`. Proposed convention for when this needs
  tightening (not yet applied to any project — adopt once a cross-domain
  project, e.g. a migrated bot, makes enforcement actually useful):
  - `type:app` / `type:lib` / `type:e2e` — what kind of project it is.
  - `scope:food-maps` / `scope:bots` / `scope:shared` — which product/domain
    it belongs to.
  - Example `depConstraints`:
    ```js
    depConstraints: [
      { sourceTag: "type:app", onlyDependOnLibsWithTags: ["type:lib"] },
      { sourceTag: "type:e2e", onlyDependOnLibsWithTags: ["type:app", "type:lib"] },
      { sourceTag: "type:lib", onlyDependOnLibsWithTags: ["type:lib"] },
      { sourceTag: "scope:food-maps", onlyDependOnLibsWithTags: ["scope:food-maps", "scope:shared"] },
      { sourceTag: "scope:bots", onlyDependOnLibsWithTags: ["scope:bots", "scope:shared"] },
    ];
    ```

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

## Python (uv + ruff)

No Python app exists in the repo yet — apps are created on demand via
`make new-bot` (see "Scaffolding" below), which is where these conventions
currently live.

- **uv**, not Poetry — matches the real standalone bot repos and eases their
  eventual migration into this monorepo (see "Scaffolding" below for detail).
- **Ruff** for lint + format. Shared rules live in a root-level `ruff.toml`;
  each generated app's `pyproject.toml` inherits them via
  `[tool.ruff] extend = "../../ruff.toml"` and can add per-app overrides
  below the `extend` line — ruff merges rather than replaces. Current shared
  rules: `target-version = "py313"`, `line-length = 100`,
  `select = ["E", "F", "I", "N", "W", "UP"]`, `quote-style = "double"`.
- Lint target (`uv run ruff check . && uv run ruff format --check .`) and
  test target (`uv run pytest`) are cached `nx:run-commands` targets — same
  non-plugin approach as Go, since there's no official Nx Python plugin
  (`@nxlv/python` was removed; see "Scaffolding" below).
- The three standalone bot repos (`index-watch`, `table-talks`,
  `otobr-buddy`) keep their own independent, near-identical ruff config —
  the shared `ruff.toml` here only covers apps scaffolded in this repo, and
  isn't wired up to them unless/until they're migrated in.

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
- Migrating `index-watch`/`table-talks`/`otobr-buddy` into this monorepo:
  one at a time, starting with the simplest, only after the generator +
  deploy pattern above have been proven with a throwaway app (they have real
  users/schedules — don't touch them as a drive-by change).
- Module boundary tags/`depConstraints` are documented as a convention (see
  "Nx conventions") but not applied to any `project.json` yet — adopt when
  the first cross-domain project (e.g. a migrated bot) makes enforcement
  useful, not speculatively now.
- Root `ruff.toml` only covers apps scaffolded by the `telegram-bot`
  generator; standalone bot repos keep their own independent config until
  migrated.
