---
name: release-and-deployment
description: How Docker builds, GHCR publishing, and nx release versioning work in this repo — the release.yml/publish.yml event chain, PAT usage, SSH commit signing, per-app Dockerfile deployment pattern, and nx.json/release.projects caveats. Load this when working on release, publish, versioning, or Docker deployment machinery.
---

# Deployment pattern

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
the old standalone `tanjd/jeddy-tan` one — see the root `CLAUDE.md`'s "Known
gaps / deferred work" for the exact settings this still needs (a manual,
one-time dashboard change).

# Release versioning (`nx release`) and publishing

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
- **PR title format is load-bearing, not cosmetic**: this repo only allows
  squash merging, and GitHub uses the PR title as the squash commit message
  whenever the PR has more than one commit (`squash_merge_commit_title:
COMMIT_OR_PR_TITLE`). `nx release`'s `conventionalCommits` engine reads
  that squash commit on `main` to decide whether a project gets a version
  bump — a title that isn't `type(scope): summary` produces a commit it
  can't parse, so the touched project(s) get **no bump, no changelog entry,
  no published image**, with no error or warning anywhere in CI. This
  happened for real: PRs #31 and #33 shipped admin dashboard/SMTP/
  email-change features to `bookshelf-backend` under plain-sentence titles
  ("Bookshelf: SMTP email, admin dashboard, mobile bottom nav"), so it stayed
  on the 0.2.0 image in production while `/admin/dashboard` 404'd — that
  route only existed in source, never in a shipped image. Fixed by manually
  cutting `bookshelf-backend@0.3.0` (see that app's `CHANGELOG.md`).
  `.github/workflows/pr-title.yml` now checks PR titles against Conventional
  Commits format, and the `create-pr` skill enforces it when drafting titles
  — but the check isn't a required status check yet (branch protection on
  `main` isn't enabled, see root `CLAUDE.md`'s "Known gaps"), so it can't
  block a merge on its own yet.
