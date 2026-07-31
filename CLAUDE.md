# core-repository

Nx monorepo (TS/Next.js + Go + Python/uv), managed with pnpm, developed inside
a devcontainer. Long-term home for side projects, starting with Telegram bots.

## Workspace layout

- `apps/food-maps` — Next.js frontend.
- `apps/food-maps-backend` — Go API (huma + chi + SQLite via `mattn/go-sqlite3`, cgo).
- `apps/food-maps-e2e` — Playwright E2E tests for `food-maps`.
- `apps/index-watch` — Telegram bot (index drawdown tracker), Python/uv,
  migrated from the standalone `tanjd/index-watch` repo (squash-imported, no
  preserved history). Same `uv` + `nx:run-commands` pattern as the
  `telegram-bot` generator's own output, just not generator-scaffolded itself
  (setuptools build-backend, not hatchling, since that's what the source
  repo used).
- `apps/table-talks` — Telegram bot (theme-based conversation card game),
  Python/uv, migrated from the standalone `tanjd/table-talks` repo
  (squash-imported, no preserved history). Same non-generator-scaffolded
  pattern as `index-watch`, with two adaptations beyond the standard
  cross-cutting drops: its `src/` package was renamed from the standalone
  repo's flat `src/` (`python -m src.index`) to `src/table_talks/`
  (`python -m table_talks.index`) to match `index-watch`'s and the
  generator's nested-package convention; and its bot-info-screen feature
  (`src/table_talks/version.py`, reads a live version + recent changelog
  entries for in-chat display) was adapted to read `package.json` (the file
  `nx release` actually keeps current) instead of `pyproject.toml`, and to
  match `nx release`'s changelog header format (`#` for the newest entry,
  `##` for older ones) — `index-watch` has no equivalent runtime feature, so
  this gap was never hit there.
- `libs/food-maps-data` — shared TS lib, consumed via the `@tanjd/food-maps-data`
  path alias in `tsconfig.base.json` (not an npm package — no `package.json` of
  its own, folded into the root pnpm workspace).
- `tools/generators/telegram-bot` — local Nx generator for scaffolding new bots.

`ledger-lens` (Next.js + FastAPI dashboard, not a bot) is queued to migrate
into this monorepo next, same as `index-watch`/`table-talks` above — a
deliberate, separate effort; do not fold it in as a drive-by change.
(`otobr-buddy`, previously a third queued bot, is no longer in the active
migration queue — superseded by `ledger-lens`.)

## Common commands

Prefer the `make` targets below over typing the raw `pnpm nx` invocations —
run `make help` for the full list.

```bash
make setup                              # pnpm install --frozen-lockfile, goimports, rtk init (Claude/Cursor)
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

`apps/index-watch` and `apps/table-talks` (both migrated) are the only
Python apps in the repo so far; further apps are created on demand via
`make new-bot` (see "Scaffolding" below) or migrated in like these were.

- **uv**, not Poetry — matches the real standalone bot repos and eases their
  eventual migration into this monorepo (see "Scaffolding" below for detail).
- **Ruff** for lint + format. Shared rules live in a root-level `ruff.toml`;
  each app's `pyproject.toml` inherits them via
  `[tool.ruff] extend = "../../ruff.toml"` and can add per-app overrides
  below the `extend` line — ruff merges rather than replaces. Current shared
  rules: `target-version = "py313"`, `line-length = 100`,
  `select = ["E", "F", "I", "N", "W", "UP"]`, `quote-style = "double"`.
  **Important**: ruff's `src` setting does _not_ propagate through `extend`
  (confirmed by reproduction, not just docs) — every app must also set its
  own `src = ["src", "tests"]` below the `extend` line, or first-party
  imports silently fail isort's import-grouping. The generator template,
  `index-watch`, and `table-talks` already do this.
- Lint target (`uv run ruff check . && uv run ruff format --check .`) and
  test target (`uv run pytest`) are cached `nx:run-commands` targets — same
  non-plugin approach as Go, since there's no official Nx Python plugin
  (`@nxlv/python` was removed; see "Scaffolding" below).
- The remaining standalone app repo (`ledger-lens`) keeps its own
  independent, near-identical ruff config — the shared `ruff.toml` here
  only covers apps in this repo, and isn't wired up to it unless/until it's
  migrated in.

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
match the standalone bot repos this pattern was proven against
(`index-watch`, `table-talks`, both now migrated). `@nxlv/python` was
removed as a dependency — nothing here uses it.

The devcontainer's `uv` feature (`ghcr.io/va-h/devcontainers-features/uv`) is
required for the generator's post-generation `uv lock` step and for the
generated targets to run at all.

## Deployment pattern

Any deployable app owns its own `Dockerfile` under `apps/<name>/`, built with
the **repo root as build context**: `make docker-build APP=<name>` (or
`docker build -f apps/<name>/Dockerfile .` — see the root `.dockerignore`,
since `node_modules` alone is 1.3G).

"Is this app dockerized" is expressed purely as an Nx fact: a project opts in
by declaring empty `docker-build`/`docker-push` target stanzas (`{}`) in its
`project.json` — the shared `nx.json` → `targetDefaults` entries supply the
actual `docker buildx build` command via `nx:run-commands`, keyed off the
`$NX_TASK_TARGET_PROJECT` env var Nx injects (which doubles as both the
`apps/<name>` path segment and the `ghcr.io/tanjd/<name>` image name, so one
shared command covers every dockerized app — no per-app duplication). Not
every app is deployable (e.g. `food-maps` isn't Dockerized, `food-maps-backend`,
`index-watch`, and `table-talks` are); the generator (`tools/generators/telegram-bot`)
scaffolds new bots with these targets already wired up. Both targets are `cache: false`
— a docker build/push has no restorable Nx artifact, and Nx-caching
`docker-push` in particular would risk silently skipping a real push; Docker's
own buildx layer cache (`--cache-from`/`--cache-to type=gha`) handles
incrementality instead.

`.github/workflows/ci.yml` runs `nx affected -t docker-build` (build-only,
verifies every affected dockerized app still builds, no registry push) on
every push/PR to `main`. Actually publishing an image is release-gated (see
"Release versioning" below) rather than tied to every push to `main` — that
detection is now just `nx affected --withTarget docker-build`, and per-app
GHA cache scoping comes from `$NX_TASK_TARGET_PROJECT` rather than a matrix
variable, so there's a single `nx affected` step instead of one GH Actions
job per app (this replaced an earlier bash/jq loop that checked
`apps/<app>/Dockerfile` for existence and fanned out over a matrix).

GHCR was chosen over Docker Hub (which the standalone bots currently use) to
reuse the `GITHUB_TOKEN` auth already set up in `ci.yml` — not a hard lock-in.

## Release versioning (`nx release`) and publishing

Versioning and image publishing are two separate, event-chained workflows,
not one job:

1. `.github/workflows/release.yml` runs via a `workflow_run` trigger keyed
   to `ci.yml`'s completion on `main` (gated with
   `if: github.event.workflow_run.conclusion == 'success'`, since
   `workflow_run` fires on both success and failure and the job has to
   check itself) — not its own `push` trigger — so a CI failure on `main`
   blocks versioning/release instead of racing it. It does **only**
   versioning: `nx release --skip-publish` (`nx.json` → `release`)
   computes each affected project's next version from Conventional Commits,
   writes `CHANGELOG.md` + the version manifest, commits with `[skip ci]`
   (so the push-back to `main` doesn't retrigger `ci.yml`, which in turn
   means it never retriggers `release.yml` either since that now depends on
   a completed `ci.yml` run — no loop), tags `<project>@<version>`
   (independent-relationship default), and creates a GitHub Release per
   project. A `workflow_dispatch` trigger is also wired up for manual runs
   (e.g. retrying after a transient failure) and bypasses the CI-conclusion
   check, since there's no `workflow_run` event to inspect in that case.
2. `.github/workflows/publish.yml` parses a release tag
   (`<project>@<version>`) to get an exact single project name, checks out
   that tag, and runs `nx run <project>:docker-push` — tagging the image
   `latest`, the commit SHA, and the released `v<semver>`, pushed to
   `ghcr.io/tanjd/<app-name>`.

`publish.yml` listens for `release: published`, which does **not** fire
for a release created via the default `GITHUB_TOKEN` — GitHub's
anti-recursion rule blocks anything the default token creates (a push, a
release, etc.) from triggering other workflows,
`workflow_dispatch`/`repository_dispatch` excepted. So `release.yml`'s
checkout and its `nx release` step both authenticate with
`secrets.GHA_TRIGGER_TOKEN` (a fine-grained PAT, repo-scoped to
`tanjd/core-repository`, `Contents: Read and write` only) instead — a PAT
acts like a real user, so the release it creates fires `release: published`
normally, no bridging step required. Same convention the standalone
`tanjd/index-watch` repo already used (there, `GHA_TRIGGER_TOKEN` backs
`python-semantic-release`'s `github_token` input). The PAT is a fine-grained
token, so it expires (max 1 year) and needs rotating before then — done
manually via GitHub UI → Settings → Developer settings → Fine-grained
tokens, then updating the `GHA_TRIGGER_TOKEN` repo secret.
`publish.yml`'s `workflow_dispatch` trigger doubles as how to retry a
publish without re-cutting the release.

Publishing is deliberately **not** `nx affected -t docker-push` off a
commit range: a `release: published` event names one exact project+version,
not a diff, and `nx affected` would be the wrong tool here anyway — the
same `nx.json`/`pnpm-lock.yaml`-is-global ripple noted below means an
unrelated commit range could mark a project "affected" that was never
actually released this time, publishing an image not backed by any real
version bump.

`food-maps-backend`, `index-watch`, and `table-talks` are versioned
independently — `release.projects` in `nx.json` is the explicit list of
which apps participate (not every project; `food-maps`/`food-maps-data`/
`food-maps-e2e` aren't deployed, so they're excluded). New bots must be
added to that list by hand when scaffolded/migrated.

- Each app carries a bare `package.json` (`{name, version, private: true}`)
  purely as the version manifest `nx release` reads/writes — it joins the
  pnpm workspace (harmless, no deps) but nothing else touches it. This
  exists because `nx release`'s default `VersionActions` implementation
  (from `@nx/js`) hard-requires a `package.json` to read/write the version;
  the alternative (the experimental `@nx/docker` plugin, or a hand-rolled
  `versionActions` implementation) was rejected as more invasive for what's
  a fairly standard polyglot-monorepo workaround.
- **Accepted caveat**: `nx release`'s conventional-commits engine reuses the
  same graph-based "affected" logic as plain `nx affected` — and `nx.json`
  changes are unconditionally treated as global by Nx core (confirmed via
  `nx show projects --affected --files=nx.json`, and via the installed
  `nx/src/config/nx-json.d.ts`: the old `implicitDependencies` field that
  could scope this is deprecated with no `namedInputs`/`sharedGlobals`
  replacement for nx.json itself — `sharedGlobals` only governs per-project
  cache-hash inputs, a different mechanism from what marks a project
  "affected"). So _any_ `nx.json` edit — not just `release.projects`
  changes — marks every project affected, both for `nx release` (giving
  every `release.projects` entry a "no code changes, aligned with other
  projects" changelog entry) and for a plain `nx affected -t lint test`
  run (reruns CI/pre-commit for the whole graph even for an unrelated
  single-line change, as happened when `table-talks` was added to
  `release.projects`). There's no config in this Nx version to scope
  nx.json's own blast radius; it's accepted as unavoidable rather than
  worked around. `pnpm-lock.yaml` used to have the same blanket effect but
  is now scoped via `pluginsConfig["@nx/js"].projectsAffectedByDependencyUpdates:
"auto"` in `nx.json` — only projects whose actual dependencies changed
  are marked affected by a lockfile diff now (verified: an unrelated JS
  dependency bump no longer force-patches every `release.projects` entry).
- `apps/index-watch/CHANGELOG.md` and `apps/table-talks/CHANGELOG.md` were
  each seeded with their standalone repo's pre-migration history (both used
  `python-semantic-release`, tag format `v{version}` — an unrelated tool to
  `nx release`) so changelog continuity survives the migration; `nx release`
  prepends new entries above it. `table-talks` actually reads this file (and
  `package.json`'s version) at runtime for its in-chat bot-info screen
  (`src/table_talks/version.py`) — unlike `index-watch`, where nothing reads
  either file — so its changelog-header regex was widened to match `nx
release`'s format (`#` for the newest entry, `##` for older ones), not
  just the carried-over `python-semantic-release` format (`##` only).

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
- Migrating standalone repos into this monorepo, one at a time (they have
  real users/schedules — don't touch them as a drive-by change): the
  generator + deploy pattern has now been proven with a throwaway app, and
  `index-watch` and `table-talks` are migrated (see `apps/index-watch` and
  `apps/table-talks` above). `ledger-lens` is still pending.
- Module boundary tags/`depConstraints` are documented as a convention (see
  "Nx conventions") but not applied to any `project.json` yet — adopt when
  the first cross-domain project (e.g. a migrated bot) makes enforcement
  useful, not speculatively now.
- Root `ruff.toml` covers apps scaffolded by the `telegram-bot` generator
  and migrated apps (`index-watch`, `table-talks`); the remaining standalone
  repo (`ledger-lens`) keeps its own independent config until migrated.
- `nx release` is bootstrapped and running automatically on every push to
  `main` for `food-maps-backend` and `index-watch` (tags exist:
  `food-maps-backend@0.2.0`, `index-watch@{0.1.1,0.2.0,1.0.0}`).
  `table-talks` still needs its one-time first-release bootstrap **after
  this migration is merged to `main`** (not from the feature branch — the
  bootstrap tag needs to land on a commit reachable from `main`, which a
  squash-merge wouldn't guarantee for a feature-branch-run bootstrap):
  ```bash
  git checkout main && git pull
  # table-talks: pin to the version already live on Docker Hub, for continuity
  # (its bot-info screen displays this version to end users, unlike index-watch)
  npx nx release 1.5.0 --projects table-talks --first-release
  ```
  Run with `--dry-run` appended first to preview. After this one-time step,
  every subsequent push to `main` versions `table-talks` automatically too.
