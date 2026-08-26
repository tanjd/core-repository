# Bookshelf version sync spec

**Status:** Draft — for review · **Scope:** `nx.json`, `release.yml`/`publish.yml` (maybe),
`apps/bookshelf`, `apps/bookshelf-backend` · **Depends on:** existing `nx release` setup
(`projectsRelationship: "independent"` today) · **Related:**
[`upgrade-changelog-spec.md`](./upgrade-changelog-spec.md) (changelog UX — implements with
independent versions; benefits from this spec once aligned)

Bookshelf is deployed as a **stack** (frontend + backend via compose), but `nx release` versions
the two apps independently — `0.22.0` vs `0.18.0` today. Self-hosters and members see one
product; ops pulls two GHCR images. This spec aligns semver so **one Bookshelf version** maps to
one frontend tag and one backend tag.

## Why now

- Footer/changelog UX (separate spec) standardises on **app version** only; the API footnote
  still shows a different number, which reads like a bug.
- `docker compose pull` updates both images, but release tags (`bookshelf@0.22.0`,
  `bookshelf-backend@0.18.0`) don't communicate "same release".
- Backend-only fixes that don't bump the frontend leave self-hosters on a newer API with no
  changelog entry and no upgrade banner (upgrade spec detects frontend semver only).
- `ledger-lens` + `ledger-lens-backend` have the same independent-versioning setup — **do not**
  change them in this spec; Bookshelf only.

## Goals

- **`bookshelf` and `bookshelf-backend` share one semver** on every release going forward.
- **One logical release** bumps both `package.json` files and both `CHANGELOG.md` files together.
- **GHCR tags** for both images use the same `v<semver>` (existing `publish.yml` behaviour,
  unchanged tag shape).
- **No retroactive changelog rewrite** — historical entries stay as-is.

## Non-goals

- Syncing `bookshelf-e2e` — no `package.json` version manifest; not in `release.projects`.
- Syncing `ledger-lens` / `ledger-lens-backend` or any other app pair.
- A shared root `VERSION` file both Dockerfiles read (release group is enough).
- Changing how Immich-style "product version" is displayed in the app (covered by upgrade-changelog
  spec).
- Eliminating "version bump only" changelog entries — fixed groups generate these by design when
  one side had no code changes; acceptable.

## Current state

| Piece                                 | Today                                                                      |
| ------------------------------------- | -------------------------------------------------------------------------- |
| `apps/bookshelf/package.json`         | `0.22.0`                                                                   |
| `apps/bookshelf-backend/package.json` | `0.18.0`                                                                   |
| `nx.json` → `release.projects`        | Both listed, `projectsRelationship: "independent"` (top-level)             |
| Release tags                          | `bookshelf@<ver>`, `bookshelf-backend@<ver>` (separate GitHub Releases)    |
| `publish.yml`                         | One `docker-push` per release tag; parses `project@version`                |
| Docker image tags                     | `ghcr.io/tanjd/bookshelf:v<ver>`, `ghcr.io/tanjd/bookshelf-backend:v<ver>` |

Cross-cutting PRs already touch both changelogs; semver drift is from independent bumps when only
one project had conventional-commit-worthy changes.

## Recommended approach: Nx release group (fixed)

Use an nx **release group** with `projectsRelationship: "fixed"` covering both deployable
projects. Nx docs:
[Release Groups](https://nx.dev/docs/guides/nx-release/release-groups).

When any project in the group needs a version bump, **both** bump to the same semver. Projects
with no direct changes get a configurable "version bump only" changelog entry (already familiar
from existing `CHANGELOG.md` history).

### Proposed `nx.json` change

Move `bookshelf` and `bookshelf-backend` out of the top-level `release.projects` list into a
dedicated group. All other release projects stay in the implicit default group (unchanged).

```jsonc
{
  "release": {
    // ... existing changelog, git, version config unchanged ...
    "projects": [
      "food-maps-backend",
      "index-watch",
      "table-talks",
      "ledger-lens-backend",
      "ledger-lens",
      "otobr-buddy",
      // bookshelf + bookshelf-backend removed — now in group below
    ],
    "projectsRelationship": "independent",
    "groups": {
      "bookshelf": {
        "projects": ["bookshelf", "bookshelf-backend"],
        "projectsRelationship": "fixed",
      },
    },
  },
}
```

Verify against installed Nx (`22.7.8`) with `nx release --dry-run --groups=bookshelf` before
merging — release-group config evolved across 22.x; adjust if the schema differs.

### Initial alignment (one-time)

On the PR that introduces the group, **set both `package.json` versions to the same value**
without rewriting old changelog sections:

1. Pick the next semver explicitly — recommend **`0.23.0`** (clean break above both current
   numbers, avoids implying backend "caught up" to 0.22.0 without a release).
2. Manually set `apps/bookshelf/package.json` and `apps/bookshelf-backend/package.json` to
   `0.23.0`.
3. Do **not** edit historical `CHANGELOG.md` entries.
4. First grouped `nx release` run after merge produces the first synchronised release from that
   baseline.

Alternative: align to `0.22.0` (max of the two) — simpler narrative ("backend catches up") but
implies a backend release at 0.22.x that never existed as a tagged image. **Prefer 0.23.0.**

### Release tags and `publish.yml`

Fixed groups still create **per-project tags** by default (`bookshelf@0.23.0`,
`bookshelf-backend@0.23.0`) unless `releaseTag.pattern` is customised. That matches today's
`publish.yml` parser (`project="${tag%@*}"`) — **no workflow change required** for v1.

Expect **two GitHub Releases** and **two `publish.yml` runs** per Bookshelf release (one per
image). Both push `v0.23.0`. Concurrency group in `publish.yml` keys off tag name, so they won't
cancel each other.

Optional later: a group-level tag pattern (e.g. `bookshelf-stack@v{version}`) and a publish job
that pushes both images from one release — nicer ergonomics, but requires `publish.yml` work;
defer unless dual releases become annoying.

### Changelog behaviour

Each project keeps its own `apps/<name>/CHANGELOG.md` (existing paths). On a backend-only fix:

- `bookshelf-backend/CHANGELOG.md` gets the feature/fix entry.
- `bookshelf/CHANGELOG.md` gets an nx-generated alignment entry (same as today's manual
  "version bump only" entries).

**Maintainer habit:** when a backend change is user-visible, add a matching bullet to
`apps/bookshelf/CHANGELOG.md` in the same PR (before release), so the app changelog page
(upgrade-changelog spec) stays meaningful — don't rely on "version bump only" alone for features.

## Alternatives considered

| Approach                                    | Verdict                                                                 |
| ------------------------------------------- | ----------------------------------------------------------------------- |
| **Fixed release group** (above)             | **Preferred** — native nx, minimal CI change                            |
| Manual "bump both package.json in every PR" | Error-prone; nx release won't enforce                                   |
| Single root `VERSION` file                  | Invasive (Dockerfiles, ldflags, next.config); no win over release group |
| Stay independent                            | Status quo; fights self-host product story                              |
| Merge backend into frontend as a monolith   | Out of question                                                         |

## Impact on other docs and specs

| Doc / system                                                         | Update                                                                                                                                     |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| [`upgrade-changelog-spec.md`](./upgrade-changelog-spec.md)           | After this lands: optional follow-up to detect API version bumps again (both tags move together, so app-only detection remains sufficient) |
| [`apps/bookshelf/README.md`](./README.md)                            | Upgrading section: "Bookshelf vX means both images tagged vX"                                                                              |
| [`apps/bookshelf-backend/CLAUDE.md`](../bookshelf-backend/CLAUDE.md) | Environment / release note                                                                                                                 |
| Root `CLAUDE.md` / release skill                                     | Mention bookshelf release group as precedent if ledger-lens ever syncs                                                                     |
| [`apps/bookshelf/TODO.md`](../TODO.md)                               | No entry today; none required                                                                                                              |

## Testing / verification

1. **Dry run:** `nx release --dry-run --groups=bookshelf` on a branch with a conventional commit
   touching `apps/bookshelf-backend/` — both projects should appear at the same next version.
2. **Dry run (frontend-only commit):** same with only `apps/bookshelf/` changed — backend should
   still bump (fixed relationship).
3. **After first real release:** confirm two GitHub Releases (`bookshelf@0.23.0`,
   `bookshelf-backend@0.23.0`), both images on GHCR at `v0.23.0`.
4. **Compose smoke test:** pull both `:v0.23.0` tags, `docker compose up`, `/health` version matches
   footer app version (modulo `v` prefix on API).

No unit tests in app code — this is release-config validation.

## Rollback

If the release group causes unexpected nx behaviour, revert the `nx.json` change and restore both
projects to top-level `release.projects`. Independent versioning resumes; no data migration.

## File checklist

| File                                             | Change                                                                                |
| ------------------------------------------------ | ------------------------------------------------------------------------------------- |
| `nx.json`                                        | Add `release.groups.bookshelf`; remove two projects from top-level `release.projects` |
| `apps/bookshelf/package.json`                    | Set to aligned semver (e.g. `0.23.0`) in same PR                                      |
| `apps/bookshelf-backend/package.json`            | Same                                                                                  |
| `.claude/skills/release-and-deployment/SKILL.md` | Note bookshelf fixed group (optional)                                                 |
| `apps/bookshelf/README.md`                       | One paragraph under Upgrading                                                         |
| `apps/bookshelf-backend/CLAUDE.md`               | Cross-link this spec                                                                  |

## Implementation order

1. Agree initial aligned semver (`0.23.0` recommended).
2. Edit `nx.json` release group + align both `package.json` files in one PR.
3. Run `nx release --dry-run --groups=bookshelf` locally or on a test branch; fix config if nx
   errors.
4. Merge to `main`; let `release.yml` cut the first grouped release (or manual `workflow_dispatch`
   if needed).
5. Verify GHCR tags + update README/CLAUDE.
6. (Optional) Revisit upgrade-changelog spec for dual-version detection — likely unnecessary.

Estimated size: small config PR (~20 lines `nx.json` + two version bumps + doc touch-ups). No
application code changes.

## Open decisions

1. **Initial semver** — `0.23.0` (recommended) vs align to current frontend `0.22.0`?
2. **Single GitHub Release** — worth custom `releaseTag` + `publish.yml` work now, or two releases
   per stack release is fine?
3. **When to implement** — before or after upgrade-changelog UX ships? Either order works;
   changelog UX is useful even with divergent versions; sync makes the API footnote less confusing.
