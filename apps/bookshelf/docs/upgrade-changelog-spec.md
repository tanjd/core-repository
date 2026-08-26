# Upgrade & changelog UX spec

**Status:** Implemented · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` ·
**Depends on:** existing footer version (`NEXT_PUBLIC_VERSION`), `GET /health` backend version,
`NotificationPanel` + announcement dismissal pattern, `CHANGELOG.md` (both apps), golang-migrate

Self-hosters need to trust upgrades the way Immich/Jellyfin users do: see what's new, know
whether a release touched the database, and get a nudge after pulling new images. Bookshelf
already ships version strings in the footer and `/health`; this spec wires them into a changelog
page, an in-app upgrade notice, and a release-notes convention for migrations.

## Why now

- Footer shows `v0.22.0` but it is inert text — no link, no context.
- After `docker compose pull && docker compose up -d`, members have no in-app signal that
  anything changed unless an admin posts an announcement manually.
- `bookshelf` (frontend) and `bookshelf-backend` (API) are versioned independently via
  `nx release`. User-facing surfaces use the **app version** only; API/schema versions are
  ops footnotes on the changelog page.
- DB migrations run automatically on backend startup (`internal/db/db.go`) but release notes
  never say when a migration ships or whether an admin should do anything beyond pulling images.

## Goals

- **Footer version links to a changelog page** showing recent release notes.
- **In-app "What's new" notice** when the running app is newer than the version this browser
  last acknowledged — surfaced in the existing notification bell panel, dismissible per browser
  (same model as announcements).
- **Migration visibility** — when a release includes a DB migration, the changelog entry calls
  it out explicitly; the changelog page shows the live schema migration number for ops sanity
  checks.
- **Zero new infrastructure** — no GitHub API calls at runtime, no new DB tables, no admin
  workflow beyond the existing `nx release` + CHANGELOG habit.

## Non-goals (v1)

- **Auto-update / Watchtower integration** — we notify; the admin still runs `docker compose pull`.
- **Per-user server-side dismissal tracking** — client-only `localStorage`, matching
  `useActiveAnnouncements`.
- **Parsing git history or PR titles at runtime** — hand-written (nx-generated) `CHANGELOG.md`
  entries remain the source of truth.
- **Syncing frontend/backend semver** — out of scope here; see
  [`version-sync-spec.md`](./version-sync-spec.md).
- **Changelog in transactional email** — no "Bookshelf was upgraded" mail blast.
- **Native mobile app update prompts** — web-only.

## Version model

Three version strings coexist. Document and display them consistently:

| Source                    | Example   | Where set                                                                      | Audience                        |
| ------------------------- | --------- | ------------------------------------------------------------------------------ | ------------------------------- |
| **App version** (primary) | `0.22.0`  | `apps/bookshelf/package.json` → `NEXT_PUBLIC_VERSION`                          | Members, footer, upgrade notice |
| **API version**           | `v0.18.0` | `apps/bookshelf-backend` Docker `-ldflags "-X main.version=…"` → `GET /health` | Admins, changelog footer, ops   |
| **Schema version**        | `13`      | golang-migrate `schema_migrations.version`                                     | Admins, migration notes         |

**Display rule:** call the product **"Bookshelf v0.22.0"** everywhere user-facing. On the
changelog page only, show a secondary line: `API v0.18.0 · schema migration 000013` fetched live
from the backend (logged-out users see app version + static changelog only; logged-in users get
the API/schema line).

**Upgrade detection** compares `NEXT_PUBLIC_VERSION` vs
`localStorage.bookshelf_last_seen_app_version` only. App-changelog-only means the notice fires
when the frontend image changes — not on a backend-only image bump with no frontend rebuild.

## Changelog content

### Source

`apps/bookshelf/CHANGELOG.md` — the file `nx release` already maintains. Do **not** read it at
runtime from disk (won't exist inside the standalone Next.js bundle). **Embed at build time.**

### Build-time generation

Add a small generator invoked before `@nx/next:build`:

```
apps/bookshelf/scripts/generate-changelog.ts
  → apps/bookshelf/src/lib/changelog.generated.ts
```

The script:

- Parses `CHANGELOG.md` using the same header rules as
  `apps/table-talks/src/table_talks/version.py` (`#` or `##` followed by semver).
- Emits the **last 15 versions** as typed JSON (version, date if present, body markdown string).
- Fails the build if `CHANGELOG.md` is missing or has no parseable versions (catch empty file
  regressions early).

Wire into Nx via `project.json`:

```json
"generate-changelog": {
  "executor": "nx:run-commands",
  "options": { "command": "pnpm exec tsx apps/bookshelf/scripts/generate-changelog.ts" }
},
"build": {
  "dependsOn": ["generate-changelog"]
}
```

(or add `generate-changelog` to the existing inferred build's `dependsOn` — exact wiring left to
implementer; must run in CI and Docker build).

`changelog.generated.ts` is gitignored; Docker build already runs `nx build bookshelf` so it
gets regenerated inside the image build.

### Rendering

New page `apps/bookshelf/src/app/changelog/page.tsx`:

- Server component reads `changelog.generated.ts` (static import).
- Renders each version as a `<section>` — title (`0.22.0`), optional date, body via
  `react-markdown` (add workspace dependency).
- Bottom of page: "Running API …" / "Schema migration …" — client component fetches
  `GET /api/health` (extend proxy to forward backend `/health` JSON, or add
  `GET /api/version` — see Backend).

### Footer

In `src/app/layout.tsx`, wrap the version span:

```tsx
<Link href="/changelog" className="hover:underline underline-offset-2">
  v{process.env.NEXT_PUBLIC_VERSION}
</Link>
```

## In-app upgrade notice

### Detection hook — `useUpgradeNotice`

New hook `apps/bookshelf/src/hooks/useUpgradeNotice.ts`:

```ts
const APP_KEY = "bookshelf_last_seen_app_version";

// On mount (logged-in or logged-out — version notice is not auth-gated):
// 1. currentApp = process.env.NEXT_PUBLIC_VERSION
// 2. lastApp = localStorage APP_KEY ?? currentApp  (first visit: no notice)
// 3. hasUpgrade = semverGt(currentApp, lastApp)
// 4. dismiss() writes currentApp to localStorage
```

Use a small `semverGt(a, b)` (major.minor.patch only — matches our tags).

**First visit:** initialise keys to current versions silently (no notice). Only upgrades after
the first load trigger the banner.

**Logged-out visitors:** still see the notice (public community may hit the site before logging
in after an upgrade). Dismissal is per-browser, not per-account.

### UI placement

Extend `NotificationPanel` with an optional `upgradeNotice` prop (parallel to `announcement`):

```
┌─────────────────────────────────┐
│ 🎉 What's new in v0.23.0        │  ← upgrade banner (new)
│    See release notes →    [×]   │
├─────────────────────────────────┤
│ 📣 Announcement (existing)      │
├─────────────────────────────────┤
│ Notifications (existing)        │
└─────────────────────────────────┘
```

- Label: **"What's new in v{currentApp}"**
- Primary action: navigate to `/changelog` (closes popover).
- Dismiss (`×`): calls `dismiss()` on the hook — does **not** mark anything server-side.
- Badge count in `NotificationBell`: add `+1` when upgrade notice visible (same as announcement).
- Visible to **all users** (logged in or not). In-app only — no email.

Wire in `NotificationBell.tsx` alongside `useActiveAnnouncements`.

**Stacking order:** upgrade notice **above** admin announcements — upgrade is time-sensitive and
auto-generated; announcements are rarer and admin-authored.

## Backend changes

### Extend health response (preferred over new route)

Today `GET /health` returns:

```json
{ "status": "ok", "version": "v0.18.0" }
```

Extend to:

```json
{
  "status": "ok",
  "version": "v0.18.0",
  "schema_version": 13
}
```

`schema_version` = golang-migrate's current version from the same `*sql.DB` pool (add a
`MigrationVersion(db *sql.DB) (uint, error)` helper in `internal/db/db.go` wrapping
`migrate.New(...).Version()` — handle `migrate.ErrNilVersion` as `0` for fresh installs).

No auth required — same as `/health`. Schema version is not sensitive.

### Frontend proxy

Replace the stub `src/app/api/health/route.ts` (currently returns `{ status: "ok" }` only) with
a proxy to `${BACKEND_URL}/health` — same pattern as `[...path]/route.ts` but keeps Docker
healthcheck working with real backend metadata. Existing compose healthcheck hits
`/api/health` on the frontend container — it continues to pass when backend is healthy.

Alternatively add `src/app/api/version/route.ts` if changing `/api/health` behaviour is risky
for the healthcheck — implementer's call; **one public JSON surface** is enough.

## Migration notes convention

When a PR adds files under `internal/db/migrations/`, the release CHANGELOG entry **must**
include a subsection:

```markdown
## 0.23.0 (2026-09-01)

### 🚀 Features

- **bookshelf-backend:** …

### Database migrations

Includes migration **000014\_** — adds `foo` column to `bar`. Automatic on startup; no manual
SQL required. Safe to upgrade from 0.22.x with the stack running (SQLite migrations are
forward-only; stop-the-world not required).
```

Guidelines for authors:

| Situation                          | Write                                                            |
| ---------------------------------- | ---------------------------------------------------------------- |
| New migration, backward-compatible | "Automatic on startup; no manual steps."                         |
| Long-running migration on large DB | "May take a few seconds on first boot; expect a single restart." |
| Requires backup first (rare)       | "Back up before upgrading." + link to backups page               |
| Irreversible schema change         | "No down migration — back up first."                             |

Optional CI guard (later): lint PR body or a `MIGRATION` label when
`apps/bookshelf-backend/internal/db/migrations/*.up.sql` changes — **not required for v1**.

Document the convention in `apps/bookshelf/README.md` § Upgrading and
`apps/bookshelf-backend/CLAUDE.md`.

## Release process (maintainer)

Unchanged mechanics (`nx release` on push to `main`). Migration subsections in
`CHANGELOG.md` are injected automatically by `tools/bookshelf-changelog/renderer.js` (see
`apps/bookshelf-backend/CLAUDE.md`); manual edits are only needed when the default boilerplate
is insufficient.

1. Spot-check the release `CHANGELOG.md` entry after `nx release` if the migration needs
   non-default wording (backup-first, long-running, irreversible).
2. Tag/publish both `bookshelf` and `bookshelf-backend` images when the release affects either
   side of a feature (already typical for cross-cutting PRs).
3. After deploy, spot-check `/changelog` and the upgrade banner on a browser that still has old
   `localStorage` keys (or clear keys once in devtools).

## File checklist

| File                                                            | Change                                       |
| --------------------------------------------------------------- | -------------------------------------------- |
| `apps/bookshelf/scripts/generate-changelog.ts`                  | **New** — parse CHANGELOG → generated TS     |
| `apps/bookshelf/src/lib/changelog.generated.ts`                 | **New**, gitignored                          |
| `apps/bookshelf/src/lib/changelog.ts`                           | **New** — types + `semverGt` helper          |
| `apps/bookshelf/src/lib/changelog.test.ts`                      | **New** — parser + semver tests              |
| `apps/bookshelf/src/app/changelog/page.tsx`                     | **New** — release notes page                 |
| `apps/bookshelf/src/hooks/useUpgradeNotice.ts`                  | **New**                                      |
| `apps/bookshelf/src/hooks/useUpgradeNotice.test.ts`             | **New**                                      |
| `apps/bookshelf/src/components/NotificationPanel.tsx`           | Upgrade banner block                         |
| `apps/bookshelf/src/components/NotificationBell.tsx`            | Wire hook, badge count                       |
| `apps/bookshelf/src/app/layout.tsx`                             | Footer link                                  |
| `apps/bookshelf/src/app/api/health/route.ts`                    | Proxy backend `/health`                      |
| `apps/bookshelf/project.json`                                   | `generate-changelog` target + build dep      |
| `apps/bookshelf/.gitignore` or root `.gitignore`                | Ignore `changelog.generated.ts`              |
| `apps/bookshelf-backend/internal/db/db.go`                      | `MigrationVersion()` helper                  |
| `apps/bookshelf-backend/cmd/server/main.go`                     | Add `schema_version` to `/health` JSON       |
| `apps/bookshelf-backend/internal/db/db_test.go` or handler test | Health JSON shape                            |
| `apps/bookshelf/README.md`                                      | Migration notes convention under Upgrading   |
| `package.json` (root)                                           | Add `react-markdown` dependency              |
| `apps/bookshelf/TODO.md`                                        | Remove or strike the version-changelog item  |
| `apps/bookshelf-e2e/src/changelog.spec.ts`                      | **New** — footer link, page renders, dismiss |

## Testing

### Unit

- `generate-changelog.ts` / parser: handles `#` and `##` headers, skips "Thank You" sections,
  extracts ≥1 version from the real `CHANGELOG.md` format.
- `semverGt`: `0.22.0 > 0.21.0`, `0.22.1 > 0.22.0`, equal → false.
- `useUpgradeNotice`: mock localStorage + fetch; first visit silent; bump triggers notice;
  dismiss writes keys.

### Backend

- `GET /health` includes `schema_version` matching applied migrations in test DB.

### E2E (`bookshelf-e2e`)

1. Footer `v…` link navigates to `/changelog`, page contains current `NEXT_PUBLIC_VERSION`.
2. Seed `localStorage` with an older app version → bell badge increments → panel shows
   "What's new" → dismiss clears badge.
3. `/changelog` renders at least one bullet from the latest release entry.

## Resolved decisions

1. **Markdown rendering** — use `react-markdown` on `/changelog`. Add to workspace root
   `package.json` (hoisted dep, same as other Next apps here).
2. **Changelog scope** — app changelog only (`apps/bookshelf/CHANGELOG.md`). API version and
   schema migration number appear as a footnote line on the page, not separate release notes.
3. **Upgrade notice audience** — **all users**, in-app only (notification panel banner). No email.
4. **Version sync** — out of scope; see [`version-sync-spec.md`](./version-sync-spec.md).

## Implementation order

1. Backend: `schema_version` on `/health` + test.
2. Build pipeline: `generate-changelog.ts` + `/changelog` page + footer link.
3. `/api/health` proxy fix.
4. `useUpgradeNotice` + `NotificationPanel` banner + tests.
5. E2E spec.
6. README / CLAUDE migration-notes convention.

Estimated size: ~400–600 LOC across both apps, no schema migration required for this feature
itself.
