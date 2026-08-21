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
- `apps/otobr-buddy` — Telegram bot (one-to-one Bible reading partnerships), Python/uv. See
  `apps/otobr-buddy/CLAUDE.md`.
- `apps/ledger-lens-backend` — Python/uv + FastAPI + SQLModel API (portfolio CSV ingestion +
  analysis). See `apps/ledger-lens-backend/CLAUDE.md`.
- `apps/ledger-lens` — Next.js + Tailwind CSS + shadcn/ui frontend for the above. See
  `apps/ledger-lens/CLAUDE.md`.
- `libs/food-maps-data` — shared TS lib, consumed via the `@tanjd/food-maps-data`
  path alias in `tsconfig.base.json` (not an npm package — no `package.json` of
  its own, folded into the root pnpm workspace).
- `libs/telegram-bot-shared` — shared Python lib (health-check server, dev/prod bot-token
  selection, `ADMIN_CHAT_IDS` allowlist parsing, logging setup) for the Telegram bots, consumed
  via a `uv` local path dependency (see `.claude/rules/python.md`).
- `apps/jeddy-tan` — personal portfolio site (React Router + MUI), plain JS on
  Vite/`@nx/vite` (unlike every other frontend here, which is Next.js). See
  `apps/jeddy-tan/CLAUDE.md`.
- `apps/bookshelf-backend` — Go API (huma + GORM/SQLite), community book-lending app. See
  `apps/bookshelf-backend/CLAUDE.md`.
- `apps/bookshelf` — Next.js + Tailwind CSS + shadcn/ui frontend for the above. See
  `apps/bookshelf/CLAUDE.md`.
- `apps/bookshelf-e2e` — Playwright E2E tests for `bookshelf` + `bookshelf-backend`, against real
  servers rather than mocks. See `apps/bookshelf-e2e/CLAUDE.md`.
- `tools/generators/telegram-bot` — local Nx generator for scaffolding new bots.

## Common commands

Prefer `make` targets over raw `pnpm nx` invocations — run `make help` for the full list.

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
- `pnpm nx affected -t lint test e2e` — the full Nx lint/test/e2e for
  whatever the commit affects, not staged-file-scoped like the two layers
  above.

The third layer duplicates what CI runs, on purpose: failing locally before a
push beats failing in CI, even with no remote cache to offset it
(`neverConnectToCloud: true` in `nx.json`) — local `.nx/cache` still speeds up
repeat commits that don't touch a given project. `e2e` in particular can add
real time to a commit that touches `bookshelf`/`bookshelf-backend`/
`bookshelf-e2e` (or `food-maps`/`food-maps-backend`/`food-maps-e2e`) — a
production `next build` plus a full Playwright run — but catching a broken
e2e spec before it reaches CI is worth that cost; `make setup` installs the
Playwright browsers this needs. CI (`.github/workflows/ci.yml`, triggered on
`push`/`pull_request` to `main`) runs both `nx affected -t lint test` and,
as a separate step, `nx affected -t e2e` as the authoritative gate, since
`--no-verify` or a merge can land changes the hook never saw. Branch protection on `main`
requires the `main`, `docker-build`, and `validate` (PR Title) checks to pass
and the PR branch to be up to date before merging — `enforce_admins` is off,
so this can still be bypassed manually if needed.

## Verifying changes

When an app has an `*-e2e` project (e.g. `apps/bookshelf-e2e`), verify a UI change by running or
extending that suite (`pnpm nx e2e <app>-e2e`) instead of writing a one-off headless-browser
script — a spec is reusable the next time this area changes; a scratch script is thrown away
after one use. `apps/bookshelf-e2e` boots real servers (backend + frontend, not route mocks) for
exactly this reason — see its `CLAUDE.md` for the pattern (Playwright's array-form `webServer`,
a seeded-auth `setup` project) to follow when bringing another app's e2e suite up to the same
standard. `apps/food-maps-e2e` exists but hasn't been upgraded yet — treat it as a template to
extend, not a finished reference.

For an app with no e2e project at all, falling back to `nx serve <app>` and a scratch script to
verify a change is still fine — the point is to reach for an existing suite before reaching for a
scratch script, not to block work on one existing everywhere.

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
  `apps/food-maps-backend/CLAUDE.md` and `.claude/rules/python.md`).
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

## Scaffolding, deployment & release

- **New bot scaffolding**: `make new-bot NAME=<bot-name>` — see the `scaffold-telegram-bot`
  skill for what it produces and its known gotchas.
- **Deployment & release**: deployable apps each own a `Dockerfile` (repo root as build
  context, `make docker-build APP=<name>`) and version independently via `nx release`,
  published to `ghcr.io/tanjd/<app>` through a release-gated GitHub Actions pipeline
  (`release.yml` → `publish.yml`). `apps/jeddy-tan` is the one exception — deployed via
  Cloudflare Pages, not Docker/GHCR. See the `release-and-deployment` skill for the full
  mechanics (event chain, PAT usage, SSH commit signing, per-app Dockerfile pattern,
  `nx.json`/`release.projects` caveats).
- **Python tooling**: uv + ruff conventions for `apps/index-watch`, `apps/table-talks`, and
  `apps/ledger-lens-backend` live in `.claude/rules/python.md` (loads automatically when
  touching those files).

## Known gaps / deferred work

- Nx is capped below 23.0.0 (currently 22.7.8), not latest — see
  `apps/food-maps-backend/CLAUDE.md` for why (its Go plugin blocks the
  upgrade). Before running `make upgrade-nx` (or `nx migrate latest`) again,
  check `npm view @nx-go/nx-go dependencies` for its `@nx/devkit` range
  first, and pin the migration to a specific 22.x version rather than
  `latest`, which may already be 23+.
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

<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

## General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax

<!-- nx configuration end-->
