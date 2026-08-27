# Setup wizard + SMTP/Google Books settings migration — spec

**Status:** Proposed · **Scope:** `apps/bookshelf` + `apps/bookshelf-backend` · **Depends on:**
existing `AppSetting` table, existing `/auth/setup`/`/auth/setup-status` flow, existing
`User.GoogleBooksAPIKey` encryption precedent (all present today — no new tables required beyond
new `AppSetting` rows)

Make first-run setup and service configuration (SMTP, Google Books API key) feel like a proper
self-hosted open-source app (Immich/Sonarr/Jellyfin): a multi-step wizard instead of a single form,
setup-time visibility into whether email and metadata lookups will work, and a migration path off
env-var-only config for those two services so they become editable from the admin UI without a
container restart.

## Why now

A prior investigation into "make this feel like a proper open-source self-hosted app" found that
first-admin bootstrap already exists and works correctly: `POST /auth/setup` (race-safe, DB
transaction), `GET /auth/setup-status`, a frontend `/setup` page, and an app-wide `SetupGuard` that
redirects the whole app to `/setup` until an admin exists. That ruled out the "build a setup wizard
from scratch" framing — the real gaps, confirmed with the app owner, are:

1. **Multi-step wizard UX** — `/setup` today is a single form (name/email/password). Tools like
   Immich walk through a welcome screen, service/environment checks, then account creation.
2. **SMTP and the Google Books API key are env-var-only**, set once at deploy time and requiring a
   container restart to change — there's no in-app way to see whether they're configured/reachable
   during setup, or to edit them afterward without touching the host's `.env`/`docker-compose.yml`
   and restarting. The ask was specifically how to **migrate** the current env-var-based config
   without breaking existing self-hosted deployments.

## Goals

- `/setup` becomes a 3-step wizard: Welcome → Environment (SMTP + Google Books status, editable) →
  Account (the existing form, unchanged behavior).
- SMTP (`host`/`port`/`username`/`password`/`from`) and the Google Books API key become DB-backed
  `AppSetting`s, editable from Admin → Settings and from the setup wizard, with changes taking
  effect immediately (no restart).
- Existing self-hosted deployments keep working unchanged through the upgrade: on first boot after
  upgrading, any `SMTP_*`/`GOOGLE_BOOKS_API_KEY` env vars that are set seed the corresponding
  `AppSetting` rows automatically. From then on the DB row is authoritative; the env var becomes
  inert (harmless to leave in place).
- The two secret-bearing values (`smtp_password`, `google_books_api_key`) are encrypted at rest and
  never returned in plaintext by any API response, matching the existing
  `User.GoogleBooksAPIKey`/`GoogleBooksKeyConfigured` convention.

## Non-goals

- **No new settings beyond SMTP + Google Books key.** Other env vars (`JWT_SECRET`,
  `ENCRYPTION_SECRET`, `DB_PATH`, `FRONTEND_ORIGIN`, `CORS_ORIGINS`) stay env-only — they're either
  security-sensitive in a way that shouldn't be DB-editable at runtime (secrets used to decrypt
  other secrets) or genuinely deploy-time/infra concerns, not per-instance service config.
- **No general-purpose "secrets in `AppSetting`" framework.** This spec adds one masking/encryption
  convention scoped to these two keys; it doesn't attempt to make the generic `GetSettings`/
  `UpsertSettings` machinery declaratively aware of arbitrary sensitive keys beyond a small
  hardcoded set (`sensitiveSettingKeys`).
- **No wizard step for anything else** (branding, initial catalog import, community name) — out of
  scope unless a future spec asks for it.

## Current mechanics this design builds on

- `AppSetting{Key, Value string}` — generic key/value table
  (`internal/repository/gorm/admin_repo.go`: `GetSettings`, `GetSetting(key)`,
  `UpsertSetting(key, value)`), seeded with defaults in `internal/db/db.go`'s `Seed()`. Exposed via
  `GET/PATCH /admin/settings` and `GET /admin/settings/export` (YAML) in
  `internal/handlers/admin.go`. **No encryption and no allowlist today** — `updateSettings` upserts
  whatever key/value pairs it's given, and `getSettings`/`exportSettings` return every stored value
  verbatim. Fine for today's settings (intervals, booleans, counts); not safe as-is for a secret.
- `models.User.GoogleBooksAPIKey` (`internal/models/models.go:21`) is the existing precedent for a
  secret stored encrypted-at-rest: `json:"-"` on the model, written via
  `encryptField(value, h.encryptionSecret)` (`internal/handlers/crypto.go`), read back via
  `decryptField`, exposed to the API only as a derived boolean (`GoogleBooksKeyConfigured`,
  `internal/handlers/auth.go:1148`) — the plaintext is write-only from the client's perspective.
  The new settings follow the same shape.
- `EmailService` (`internal/services/email.go`) and `GoogleBooksKeyPool`
  (`internal/services/google_books_keypool.go`) are constructed once in `main.go` from `cfg` (env)
  values and handed to handlers as fixed structs. `GoogleBooksKeyPool` already has an internal
  `sync.Mutex` (for rate-limit cooldown bookkeeping); `EmailService` has no locking today.
- Construction order in `main.go`: `db.Seed(database)` (line 69) → repositories including
  `adminRepo` (line 87) → `emailSvc`/`googleBooksKeyPool` (lines 94, 109) → handlers (line 133+).
  `adminRepo` is available before `emailSvc`/`googleBooksKeyPool` are built, so an env→DB seed step
  can run in between, and those two services can then be constructed by reading the (now-seeded)
  `AppSetting` values instead of `cfg` directly.
- `validateGoogleBooksAPIKey(key)` (`internal/handlers/metadata.go:312`) — existing reachability
  check (a live `GET` against the Google Books API), reusable as-is for the new health check.
- Frontend multi-step form convention: `src/app/(auth)/register/page.tsx` uses `type Step = "..."`
  - `useState<Step>` + conditionally-rendered `{step === "x" && (...)}` blocks, all in one file —
    follow this for `/setup` rather than introducing separate step-component files.
- Frontend badge vocabulary (`apps/bookshelf/CLAUDE.md` "Frontend design"): `success`/
  `destructive`/`secondary`/`outline` badges for status — reuse for "configured & reachable" /
  "not reachable" / "not configured" states in the environment step.

## New settings keys

Add to `AppSetting`: `smtp_host`, `smtp_port`, `smtp_username`, `smtp_password` (sensitive),
`smtp_from`, `google_books_api_key` (sensitive). Not added to `db.Seed()`'s static defaults list —
these are seeded from env on first boot instead (see below); `smtp_port` can still get a plain
static default (`"587"`) alongside the existing entries in `Seed()`.

## Encryption and masking

- A new `sensitiveSettingKeys = map[string]bool{"smtp_password": true, "google_books_api_key": true}`,
  in a small shared file (e.g. `internal/handlers/settings.go`) since both `AuthHandler` and
  `AdminHandler` need it.
- **On write** (`updateSettings` in `admin.go`, and the new setup-wizard save endpoint below): for
  a sensitive key with a non-empty value, encrypt via `encryptField(value, encryptionSecret)`
  before `UpsertSetting`; an explicit empty-string value clears the setting (stored empty, not
  encrypted) — same "empty clears it" convention as `applyGoogleBooksKeyUpdate`
  (`internal/handlers/auth.go:1310-1322`).
- **On read** (`getSettings`, `exportSettings`): sensitive keys' values are replaced with a fixed
  mask (`"••••••••"`) if non-empty, left empty if unset — the ciphertext and plaintext are never
  returned to any client, matching the `GoogleBooksKeyConfigured`-boolean precedent. The mask
  string must be recognized as an "unchanged" sentinel on the next `updateSettings` call for that
  key (skip the upsert entirely), so a settings-page GET→edit-something-else→PATCH round trip
  doesn't stomp the real secret with the literal mask text. Actually clearing a secret requires the
  UI to send `""` explicitly via a distinct "Clear" action, not merely leaving the masked field
  alone.
- `AdminHandler` and `AuthHandler` both need `encryptionSecret string` — `AuthHandler` already has
  it; add it to `AdminHandler`'s fields and thread it through `NewAdminHandler`
  (`admin.go:34-37`, `main.go:139`).

## Reconfigurable services

- `EmailService`: add `sync.RWMutex`-guarded fields plus
  `func (s *EmailService) Reconfigure(host, port, username, password, from string)` (write lock),
  and read-lock the existing host/port/username/password/from reads inside `SendEmail` and the new
  `CheckConnection` (below). Constructor behavior otherwise unchanged.
- `GoogleBooksKeyPool`: add `func (p *GoogleBooksKeyPool) Reconfigure(keys []string)` swapping
  `p.keys` under the existing `p.mu` lock (reuse it rather than adding a second lock). **Note for
  implementation**: `Key()` currently reads `len(p.keys)`/`p.keys[...]` outside `p.mu`, which is
  only safe while `keys` is immutable (true today, not once `Reconfigure` exists) — either move
  that read under `p.mu` too, or switch `p.keys` to an `atomic.Pointer[[]string]`-style swap.
  Confirm which is less invasive once implementing.
- Both services' constructors in `main.go` change from reading `cfg.SMTP*`/`cfg.GoogleBooksAPIKeys`
  directly to reading the (post-seed) `AppSetting` values via `adminRepo.GetSetting(...)`, falling
  back to the `cfg` env value only if the setting is empty (defensive — covers a DB that predates
  the seed step, e.g. tests that construct these services directly without going through
  `main.go`).
- `updateSettings` (`admin.go`) and the new setup-wizard save endpoint both call
  `emailSvc.Reconfigure(...)`/`googleBooksKeyPool.Reconfigure(...)` after a successful upsert of
  the relevant keys, so a saved change takes effect immediately with no restart. `AdminHandler` and
  `AuthHandler` need direct references to both services (`AuthHandler` already holds
  `googleBooksKeys`/`email`; add `email *services.EmailService` to `AdminHandler` too, alongside
  its existing `googleBooksKeyPool` field).

## Env → DB migration

New step in `main.go`, right after `db.Seed(database)` and before `emailSvc`/`googleBooksKeyPool`
are constructed — e.g. `seedSettingsFromEnv(adminRepo, cfg, encryptionSecret)`:

- For `smtp_host`, `smtp_port`, `smtp_username`, `smtp_from`: if no `AppSetting` row exists yet for
  that key (first boot on this DB after upgrading to this version) **and** the corresponding `cfg`
  env value is non-empty, `UpsertSetting` it as plaintext.
- For `smtp_password` and `google_books_api_key`: same "no row yet + env value present" check, but
  encrypt via `encryptField` before storing.
- After this one-time seed, the DB row is authoritative — the env var becomes inert (harmless to
  leave unchanged in `.env`/`docker-compose.yml`; it's simply not read again once a row exists).
  This mirrors how `db.Seed()` already uses `FirstOrCreate` — same idempotent "only fill in what's
  missing" shape, just sourced from env instead of a hardcoded default.
- `.env.compose.example` / README get a note that these two settings are now edited in
  Admin → Settings after first boot, with the env vars documented as "used to seed the initial
  value only."

## New setup-wizard endpoints (backend)

The wizard's environment step needs to both show status and let the operator save values, before
an admin account (and thus a JWT) necessarily exists — so it can't reuse the authenticated
`PATCH /admin/settings`. Add to `internal/handlers/auth.go`, both gated the same way as the
existing `/auth/setup`/`/auth/setup-status` (403 "setup already complete" once `HasAdmin()` is
true, so neither leaks config nor accepts writes after go-live):

- `GET /auth/setup/environment` — returns `{ smtp_configured, smtp_reachable, smtp_host,
google_books_configured, google_books_reachable }`. `smtp_reachable` via a new
  `EmailService.CheckConnection(ctx context.Context) error` — short-timeout
  `net.DialTimeout("tcp", host:port, 3*time.Second)`, no auth/EHLO negotiation, a reachability
  probe only, read-locked against concurrent `Reconfigure`. `google_books_reachable` via the
  existing `validateGoogleBooksAPIKey`.
- `POST /auth/setup/environment` — accepts the same field set as the sensitive-settings slice
  above (host/port/username/password/from, google_books_api_key), applies the same
  encrypt-on-write logic as `updateSettings`, and calls `Reconfigure` on both services. Implemented
  as a slice of `AdminHandler.updateSettings`'s logic reused via the shared helper in
  `internal/handlers/settings.go`, not duplicated inline in both handlers.

## Frontend changes (`apps/bookshelf`)

- `src/lib/api.ts`: `setupEnvironment(): Promise<SetupEnvironment>` (GET) and
  `saveSetupEnvironment(input): Promise<SetupEnvironment>` (POST), plus the `SetupEnvironment`
  type — same shape/placement as the existing `setupStatus`/`setup` functions.
- `src/app/setup/page.tsx` — rework into a 3-step wizard (`type Step = "welcome" | "environment" |
"account"`), following the `register/page.tsx` in-file-step convention:
  - **Welcome**: short intro, reuse existing `BookOpen`/Card header, "Continue" → `environment`.
  - **Environment**: on mount, `api.setupEnvironment()`; renders SMTP + Google Books status
    (badges per the vocabulary above), each with an editable form section (host/port/username/
    password/from; API key) and a "Save" action hitting `saveSetupEnvironment` — explicitly
    optional ("Skip for now" alongside "Save & continue"), since these are non-blocking for account
    creation and remain editable later. "Continue" always enabled regardless of status.
  - **Account**: the existing form (name/email/password/confirm, `PasswordStrengthMeter`,
    `validatePassword`) moves here unchanged; submit behavior unchanged (`api.setup(...)` → store
    token/user → redirect to `/admin/users`).
  - Keep the existing `useEffect` gate (`api.setupStatus()` → redirect to `/login` if setup's
    already done) regardless of which step is showing.
- `src/app/admin/settings/page.tsx` gains SMTP + Google Books sections using the same masked-value
  display/edit pattern (show the mask if configured, a "Clear" control to explicitly unset, save
  via the existing `PATCH /admin/settings`) — this is where an operator manages these post-setup.
- No changes needed to `SetupGuard.tsx`, `AdminGuard.tsx`, or the backend's `SetAuth`/`RequireAdmin`
  middleware — the gating mechanism is unchanged, only what's editable inside `/setup` and
  Admin → Settings.

## Docs

- `apps/bookshelf-backend/CLAUDE.md` — new "## First-run setup" section documenting
  `/auth/setup-status` → `/auth/setup` → `/auth/setup/environment` → `SetupGuard` (the first three
  are undocumented there today despite existing and working), plus a note on the sensitive-settings
  encryption/masking convention for future settings that need it.
- `apps/bookshelf/README.md` / `.env.compose.example` — note that `SMTP_*`/`GOOGLE_BOOKS_API_KEY`
  now seed the initial DB value on first boot after upgrade, and are edited via Admin → Settings
  (or the setup wizard, for a fresh install) from then on.

## Verification (once implemented)

- Go tests: `setupEnvironment`/`saveSetupEnvironment` (configured vs. unconfigured, post-setup
  403), the encrypt/mask round trip in `updateSettings`/`getSettings`,
  `EmailService.CheckConnection`/`Reconfigure`, `GoogleBooksKeyPool.Reconfigure`, and the env→DB
  seed step's idempotency (a second boot doesn't overwrite a since-edited DB value).
- Manual: wipe local `DB_PATH`, boot with `SMTP_HOST`/`GOOGLE_BOOKS_API_KEY` set in `.env`, confirm
  they appear seeded (masked) in the wizard's environment step and in Admin → Settings after setup;
  edit a value in Admin → Settings and confirm it takes effect without a restart (e.g. re-run the
  Google Books reachability check).
- `pnpm nx affected -t lint test` for `bookshelf` and `bookshelf-backend`, per this repo's standing
  CI gate.
