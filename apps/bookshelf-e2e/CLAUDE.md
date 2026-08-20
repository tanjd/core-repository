# CLAUDE.md

Guidance for `apps/bookshelf-e2e` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Playwright e2e suite for `apps/bookshelf` + `apps/bookshelf-backend`. `playwright.config.ts` boots
**both real servers**, not mocks: `bookshelf-backend` via `go run ./cmd/server` (port 8000, DB
wiped and re-migrated on every run via `DB_PATH=./data/e2e.db`) and `bookshelf` via `next dev`
(port 3000). The frontend's `BACKEND_URL` defaults to `http://localhost:8000`, which is already
where the e2e backend listens, so no extra env wiring connects the two.

## Use this suite instead of ad hoc scripts

When verifying a bookshelf UI change works, run or extend this suite (`pnpm nx e2e
bookshelf-e2e`) rather than writing a one-off headless-browser script. A throwaway script
re-derives the same boilerplate — launch browser, log in, navigate, assert — every session and
leaves nothing behind; a spec here does the same verification once and stays as a regression
test. If the flow you're checking has no coverage yet, add a spec (or extend an existing one)
rather than reaching for a standalone script.

## Mobile viewport coverage

`projects` in `playwright.config.ts` runs every spec twice: once as `chromium` (`Desktop Chrome`)
and once as `Mobile Chrome` (`devices["Pixel 7"]`, a 412×839 viewport with touch/mobile UA).
Deliberately still Chrome, not WebKit — using a `devices` entry whose `defaultBrowserType` is
`chromium` means mobile coverage doesn't need a second browser binary beyond the `chromium` one
CLAUDE.md already has devs install. Bookshelf's UI has responsive (mobile nav, stacked layouts)
behavior that only the `Mobile Chrome` project exercises, so a spec that passes on `chromium`
alone hasn't proven the mobile layout works — check both when a UI change touches layout.

## How authentication works

`src/auth.setup.ts` is a Playwright "setup" project (`playwright.config.ts`'s `projects` array —
`chromium` and `Mobile Chrome` both declare `dependencies: ["setup"]`) that runs once both
webServers report healthy. It
`POST`s directly to `http://localhost:8000/auth/setup` to create the first admin account
(`E2E_ADMIN_EMAIL`/`E2E_ADMIN_PASSWORD`/`E2E_ADMIN_NAME`, in `src/test-users.ts` — kept out of
`auth.setup.ts` itself since Playwright disallows importing one test file from another) — the
one `auth.go` endpoint that issues a working account without an email-OTP round trip. It
tolerates a 403 ("setup already complete") so re-runs against a `reuseExistingServer` backend in
local dev (where the DB isn't wiped) stay idempotent.

Specs that need to be logged in should import those constants and log in through the UI (see
`login.spec.ts`) rather than re-deriving credentials. There's no shared `storageState` yet — add
one (save it from `auth.setup.ts`, load it via a project's `use.storageState`) once more than one
spec needs an authenticated session; not worth it for a single spec.

## Coverage

Login→catalog is currently the only core path covered. Other core flows — book request/loan,
wishlist fulfillment, admin approval — have no e2e coverage yet; extend this suite when touching
those rather than leaving them un-tested.

## Running locally

```bash
pnpm exec playwright install --with-deps chromium  # once, or after a Playwright version bump
pnpm nx e2e bookshelf-e2e
```

`make setup` (the devcontainer's `postCreateCommand`) already runs the Playwright install step,
so a normal devcontainer session shouldn't need it manually.

## CI

`nx affected -t e2e` runs in `.github/workflows/ci.yml`'s `main` job, gated the same way as
`lint`/`test` — it only executes when something in `bookshelf-e2e`'s dependency graph changed.
`project.json`'s `implicitDependencies` lists both `bookshelf` and `bookshelf-backend` so a
backend-only change triggers it too (there's no source-level import to infer that dependency
from, since Playwright drives the backend over HTTP rather than importing it — same reasoning as
`apps/food-maps-e2e`'s `implicitDependencies`).
