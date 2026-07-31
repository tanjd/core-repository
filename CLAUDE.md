# core-repository

Nx monorepo (TS/Next.js + Go + Python/uv), managed with pnpm, developed inside
a devcontainer. Long-term home for side projects, starting with Telegram bots.

## Workspace layout

Each non-trivial app under `apps/` has its own `apps/<name>/CLAUDE.md` with migration history,
adaptations, and app-specific gotchas — Claude Code loads a nested `CLAUDE.md` automatically once
work touches that directory, so it costs nothing when working elsewhere. This file stays scoped
to conventions that apply regardless of which app you're in (Nx, Go/Python tooling, deployment,
release process); the list below is just an index into the per-app files.

- `apps/food-maps` — Next.js frontend. See `apps/food-maps/CLAUDE.md`.
- `apps/food-maps-backend` — Go API (huma + chi + SQLite via `mattn/go-sqlite3`, cgo). See
  `apps/food-maps-backend/CLAUDE.md`.
- `apps/food-maps-e2e` — Playwright E2E tests for `food-maps`.
- `apps/index-watch` — Telegram bot (index drawdown tracker), Python/uv. See
  `apps/index-watch/CLAUDE.md`.
- `apps/table-talks` — Telegram bot (theme-based conversation card game), Python/uv. See
  `apps/table-talks/CLAUDE.md`.
- `apps/ledger-lens-backend` — Python/uv + FastAPI + SQLModel API (portfolio CSV ingestion +
  analysis). See `apps/ledger-lens-backend/CLAUDE.md`.
- `apps/ledger-lens` — Next.js + Tailwind CSS + shadcn/ui frontend for the above. See
  `apps/ledger-lens/CLAUDE.md`.
- `libs/food-maps-data` — shared TS lib, consumed via the `@tanjd/food-maps-data`
  path alias in `tsconfig.base.json` (not an npm package — no `package.json` of
  its own, folded into the root pnpm workspace).
- `libs/telegram-bot-shared` — shared Python lib (health-check server, dev/prod bot-token
  selection, logging setup) for the Telegram bots, consumed via a `uv` local path dependency
  (see "Python (uv + ruff)" below).
- `apps/jeddy-tan` — personal portfolio site (React Router + MUI), plain JS on
  Vite/`@nx/vite` (unlike every other frontend here, which is Next.js). See
  `apps/jeddy-tan/CLAUDE.md`.
- `tools/generators/telegram-bot` — local Nx generator for scaffolding new bots.

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

- `lint-staged`: ESLint `--fix` + oxfmt on staged JS/TS, oxfmt alone on
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
- **Inferred tasks**: `@nx/next`, `@nx/vite`, `@nx/playwright`, `@nx/eslint`,
  `@nx/jest`, and `@nx-go/nx-go` are registered Nx plugins (`nx.json` →
  `plugins`) that auto-register targets from config files already present in
  a project (`next.config.js`, `vite.config.js`, `playwright.config.ts`,
  `eslint.config.mjs`, `jest.config.ts`) — no need to hand-write those
  targets for a new TS app/lib. Go and Python have no such plugin coverage
  for their language-specific targets, so those are hand-defined
  `nx:run-commands` targets in `project.json` instead (see
  `apps/food-maps-backend/CLAUDE.md` and the Python section below).
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
      {
        sourceTag: "type:e2e",
        onlyDependOnLibsWithTags: ["type:app", "type:lib"],
      },
      { sourceTag: "type:lib", onlyDependOnLibsWithTags: ["type:lib"] },
      {
        sourceTag: "scope:food-maps",
        onlyDependOnLibsWithTags: ["scope:food-maps", "scope:shared"],
      },
      {
        sourceTag: "scope:bots",
        onlyDependOnLibsWithTags: ["scope:bots", "scope:shared"],
      },
    ];
    ```

## Python (uv + ruff)

`apps/index-watch`, `apps/table-talks`, and `apps/ledger-lens-backend` (all
migrated) are the only Python apps in the repo so far; further apps are
created on demand via `make new-bot` (see "Scaffolding" below) or migrated
in like these were.

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
  own `src` below the `extend` line, relative to that app's actual package
  layout (see each app's own `CLAUDE.md` for its specific value), or
  first-party imports silently fail isort's import-grouping.
- Lint target (`uv run ruff check . && uv run ruff format --check .`) and
  test target (`uv run pytest`) are cached `nx:run-commands` targets — same
  non-plugin approach as Go, since there's no official Nx Python plugin
  (`@nxlv/python` was removed; see "Scaffolding" below).
- **Shared code** (`libs/telegram-bot-shared`, used by both Telegram bots and the
  `telegram-bot` generator) is consumed via a `uv` local **path dependency**, not a uv
  workspace — each app keeps its own independent `uv.lock` and independent `nx release`
  versioning, consistent with every other Python app here. The convention: the consuming
  app's `pyproject.toml` declares the lib in `dependencies` and adds
  `[tool.uv.sources] <lib-name> = { path = "../../libs/<lib-name>", editable = true }`.
  Nx has no Python import-graph plugin, so the dependency edge is declared explicitly via
  `implicitDependencies` in the consuming app's `project.json` (same mechanism
  `apps/food-maps-e2e` uses for its non-import dependency on `food-maps`) — this is what
  lets `nx affected` correctly flag consumers when the shared lib changes.
  **Operational gotcha**: this doesn't auto-relock consumers — whenever
  `libs/telegram-bot-shared`'s source changes, run `uv lock` in every consuming app before
  its next `--frozen` sync (Docker build or CI) will succeed. A consuming app's Dockerfile
  also can't flatten `apps/<name>` straight to `/app` the way non-lib-consuming Python apps
  do — the `../../libs/<lib-name>` path has to resolve identically inside the image as it
  does on the host, so the image mirrors the `apps/<name>` + `libs/<lib-name>` depth under
  `/app` instead (see `apps/index-watch/Dockerfile` or `apps/table-talks/Dockerfile`).

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

Produces `apps/<bot-name>/` with a `python-telegram-bot` skeleton (health-check
endpoint, bot-token selection, and logging setup all wired in from
`libs/telegram-bot-shared` — see "Python (uv + ruff)" above), `Dockerfile`,
`.env.example`, `README.md`, and a `project.json` with
`build`/`serve`/`test`/`lint`/`docker-build` targets
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
`index-watch`, `table-talks`, `ledger-lens-backend`, and `ledger-lens` are);
the generator (`tools/generators/telegram-bot`) scaffolds new bots with these
targets already wired up. `ledger-lens`'s `Dockerfile` established a new
sub-pattern the first three dockerized apps didn't need, since every TS app's
`package.json` here is a bare version manifest with no dependencies of its
own (see "Release versioning" below) — see `apps/ledger-lens/CLAUDE.md` for
the specifics. Both `docker-build`/`docker-push` targets are `cache: false`
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

`apps/jeddy-tan` is the one exception to the Docker/GHCR pattern above: it's
a static site deployed via Cloudflare Pages' own Git integration, which
watches a repo directly and builds outside this repo's GitHub Actions
entirely — no `docker-build`/`docker-push` targets, no GHCR image. Cloudflare
Pages has native monorepo support (confirmed against its current docs), but
it needs the project's dashboard settings pointed at this repo rather than
the old standalone `tanjd/jeddy-tan` one — see "Known gaps / deferred work"
for the exact settings this still needs (a manual, one-time dashboard change).

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

`release.yml`'s "Configure git identity" step commits release changes as
tanjd's own account (`tanjd <42729752+tanjd@users.noreply.github.com>`), not
`github-actions[bot]` — followed by a "Configure SSH commit signing" step
that points `user.signingkey` at a private key written from the
`SSH_PRIVATE_SIGNING_KEY` secret and sets `gpg.format ssh` +
`commit.gpgsign true`, so the commit comes out Verified on GitHub. This is
the only way to get a Verified badge here: GitHub validates a signature
against a signing key registered on a real account, and a bot identity has
no account settings to register one against. Same convention as
`index-watch`/`table-talks`, which sign the same way via
`python-semantic-release`'s `ssh_private_signing_key`/`git_committer_*`
inputs, reusing the same key pair (`SSH_PRIVATE_SIGNING_KEY`/
`SSH_PUBLIC_SIGNING_KEY` — one signing key per person, not per repo).

Publishing is deliberately **not** `nx affected -t docker-push` off a
commit range: a `release: published` event names one exact project+version,
not a diff, and `nx affected` would be the wrong tool here anyway — the
same `nx.json`/`pnpm-lock.yaml`-is-global ripple noted below means an
unrelated commit range could mark a project "affected" that was never
actually released this time, publishing an image not backed by any real
version bump.

`food-maps-backend`, `index-watch`, `table-talks`, `ledger-lens-backend`, and
`ledger-lens` are versioned independently — `release.projects` in `nx.json`
is the explicit list of which apps participate (not every project;
`food-maps`/`food-maps-data`/`food-maps-e2e` aren't deployed, so they're
excluded). New bots/apps must be added to that list by hand when
scaffolded/migrated.

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

## Known gaps / deferred work

- Nx is capped at 22.2.2, not latest — see `apps/food-maps-backend/CLAUDE.md`
  for why (its Go plugin blocks the upgrade). Before running `make upgrade-nx`
  (or `nx migrate latest`) again, check `npm view @nx-go/nx-go dependencies`
  for its `@nx/devkit` range first.
- GitHub branch protection on `main` (requiring the CI check before merge)
  hasn't been enabled — it's a repo-settings change, not a code change.
- Module boundary tags/`depConstraints` are documented as a convention (see
  "Nx conventions") but not applied to any `project.json` yet — adopt when
  the first cross-domain project (e.g. a migrated bot) makes enforcement
  useful, not speculatively now.
- Root `ruff.toml` covers apps scaffolded by the `telegram-bot` generator
  and all migrated apps (`index-watch`, `table-talks`, `ledger-lens-backend`).
- Prettier has been replaced by `oxfmt` (Rust-based, from the oxc project) for
  JS/TS/JSON/MD/YAML/CSS/HTML formatting — see `.oxfmtrc.json` and the
  `lint-staged`/`format` wiring in `package.json`. ESLint is unchanged for now;
  an ESLint → oxlint follow-up is planned separately, blocked on picking an Nx
  integration path (no official Nx plugin yet, only the third-party
  `nx-oxlint` package) and on `@nx/enforce-module-boundaries` /
  `@nx/dependency-checks` having no oxlint equivalent (the former is already a
  no-op per the "Module boundary tags" gap above, so dropping it is low-risk;
  the latter, used in `libs/food-maps-data`, would just be dropped too).
- `apps/jeddy-tan`'s Cloudflare Pages project still points at the old
  standalone `tanjd/jeddy-tan` repo — needs a one-time manual dashboard
  reconfiguration after this migration is merged to `main` (do this **last**,
  only once `apps/jeddy-tan` actually exists on `main` — flipping the source
  repo any earlier breaks the next auto-build and takes the live site down).
  In the `jeddy-tan` Pages project's Settings → Builds & deployments (this is
  classic Pages, not the newer Workers Builds product):
  - Re-point the connected Git repo to `tanjd/core-repository`, production
    branch `main`.
  - Framework preset: `None` (a preset like "Vite" prefills a bare `dist`
    path assuming a non-monorepo layout — wrong here).
  - Root directory: leave blank/repo root, **not** `apps/jeddy-tan` —
    dependencies are hoisted to the workspace root, same reasoning as
    `ledger-lens`'s Dockerfile (see `apps/ledger-lens/CLAUDE.md`).
  - Build command: `pnpm install --frozen-lockfile && pnpm exec nx build jeddy-tan`
  - Build output directory: `dist/apps/jeddy-tan`
  - Build watch paths (separate section, same page): include
    `apps/jeddy-tan/**`, `package.json`, `pnpm-lock.yaml`, `nx.json`, so
    unrelated app changes don't trigger a redeploy.
  - Verify with a manual retry-deployment before trusting it to auto-build.
