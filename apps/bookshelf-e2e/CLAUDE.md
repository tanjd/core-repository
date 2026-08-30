# CLAUDE.md

Guidance for `apps/bookshelf-e2e` specifically — see the repo-root `CLAUDE.md` for cross-cutting
conventions (Nx, deployment, release process).

Playwright e2e suite for `apps/bookshelf` + `apps/bookshelf-backend`. `playwright.config.ts` boots
**both real servers**, not mocks: `bookshelf-backend` via `go run ./cmd/server` (port 8000, DB
wiped and re-migrated on every run via `DB_PATH=./data/e2e.db`) and `bookshelf` via a production
`next build && next start` (port 3000), not `next dev` — see "Frontend runs a production build,
not `next dev`" below for why. The frontend's `BACKEND_URL` defaults to `http://localhost:8000`,
which is already where the e2e backend listens, so no extra env wiring connects the two.

## Frontend runs a production build, not `next dev`

`next dev` compiles routes on demand and disposes/recompiles ones another browser context (or an
earlier navigation in the same one) already touched. A hard navigation landing mid-recompile could
settle onto a stale or partially-hydrated bundle, or `page.goto()` could throw outright
(`net::ERR_ABORTED`) when the dev server aborted the in-flight compile. This was the dominant
source of CI flakiness in this suite — it kept resurfacing in new forms (a stale-bundle reload
retry, then a separate goto retry for the abort case, then per-assertion timeout bumps for
post-submit `router.push` transitions) faster than each fix could be written, because every new
hard navigation or client-side transition was a fresh place for the same underlying race to show
up. Building the frontend once via `next build` and serving it with `next start` removes on-demand
compilation entirely, so specs use plain `page.goto()` + default-timeout assertions — no
retry/reload helpers, no bumped timeouts.

## The build runs as an Nx pre-step, not inside webServer

`next build` runs as the Nx target `bookshelf-e2e:e2e-build` (defined in `project.json`), which
`bookshelf-e2e:e2e` lists as a `dependsOn`. The `e2e-build` target is Nx-cached against
`apps/bookshelf` source inputs — on a cache hit (e.g. a bookshelf-backend-only change where no
frontend source changed) it restores the `.next` directory instantly, and `playwright.config.ts`'s
`webServer` entry only runs `next start apps/bookshelf --port 3000` (boots in under 2s).
Previously the build lived inside the `webServer` command, making it invisible to Nx and forcing a
fresh 30–90s `next build` on every CI run regardless of whether the frontend changed.

`e2e-build` still runs `generate-changelog`'s underlying script before `next build`. This looks
redundant next to `apps/bookshelf/project.json` declaring `generate-changelog` as a `dependsOn`
of `build`/`test`/`lint` — it isn't: `e2e-build` doesn't chain through `bookshelf:build`, so it
never picks up that `dependsOn` wiring. Without the explicit call,
`src/lib/changelog.generated.ts` (gitignored, imported by `NotificationBell.tsx` which every page
renders via `NavBar`) would be missing whenever this suite runs without bookshelf's own Nx targets
also running first — e.g. `nx affected` on a bookshelf-backend-only change. That gap broke CI the
first time a backend-only PR shipped after `NotificationBell.tsx` started importing the generated
file (PR #86) — don't remove it as "already handled by Nx."

## Use this suite instead of ad hoc scripts

When verifying a bookshelf UI change works, run or extend this suite (`pnpm nx e2e
bookshelf-e2e`) rather than writing a one-off headless-browser script. A throwaway script
re-derives the same boilerplate — launch browser, log in, navigate, assert — every session and
leaves nothing behind; a spec here does the same verification once and stays as a regression
test. If the flow you're checking has no coverage yet, add a spec (or extend an existing one)
rather than reaching for a standalone script.

`src/tools/generate-landing-screenshots.ts` is the one deliberate exception: it's not a
regression check but an asset generator, re-run whenever a catalog/nav UI change makes
`apps/bookshelf/public/screenshots/catalog-{desktop,mobile}{,-dark}.png` (embedded by
`LandingPage.tsx`) stale. Named without `.spec.ts` on purpose so `testMatch` never picks it up as
a test. It seeds a throwaway _second_ user to own the demo catalog (not the admin account it logs
in as) so no card ends up wrongly badged "Yours", mixes real ISBNs (real Open Library cover art)
with a couple of cover-less entries (BookCoverFallback's gradient), and screenshots each
theme/viewport combo at the exact width/height/deviceScaleFactor the `<Image>` calls in
`LandingPage.tsx` expect. Run it against a disposable `DB_PATH` (its header comment has the exact
commands) — never a real dev DB — and pipe the output through `pngquant --quality=70-90 --speed 1
--strip` before committing; Playwright's raw PNG output ran ~2.5x larger than the optimized file.

## Real backend vs. mocked API: which for a new spec?

Two tiers already exist in this app; a third is available but unused so far:

| Tier                      | Tool                                                 | Backend                                                                        | Use for                                                                                                                                                                                                             |
| ------------------------- | ---------------------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Component/unit            | Jest + RTL (`*.test.tsx` next to the component/hook) | `jest.mock("@/lib/api")`                                                       | Pure frontend logic — a hook's state machine, a component's conditional rendering — see `useOwnedBookIds.test.ts` or `BookCard.test.tsx`                                                                            |
| UI flow (mocked API)      | Playwright, `page.route()` intercepting `/api/**`    | Mocked                                                                         | Fast, deterministic coverage of a UI state that's expensive or awkward to produce from real data (pagination with many pages, a specific error toast, a loading/empty state, a third-party metadata API being down) |
| E2E (this suite, default) | Playwright                                           | Real (`go run ./cmd/server` + production `next build`/`next start`, see above) | Anything that exercises auth, a business rule enforced server-side, or the frontend↔backend contract itself                                                                                                         |

**Default to real backend** (the existing convention) for a new spec in this suite. Only reach for
`page.route()` mocking when the specific UI state you're testing is genuinely hard to produce with
real data — e.g. seeding 50 books to test pagination, or forcing a 500 to test an error toast —
and the thing under test is purely how the UI _renders_ a given API response, not whether the
backend actually produces that response. No spec here does this yet; if you add one, keep it in a
separate file from the real-backend specs so `grep`ing "does this hit a real server" stays a
one-line answer per file, and note in the spec's header comment why mocking was chosen over real
data.

**Don't mock a spec that's actually testing the contract or a business rule** — that's exactly
what this suite exists to catch, and two real bugs are why: `book-cover-fallback.spec.ts` was
originally written to `POST /books` without ever creating a copy, silently relying on the frontend
believing the book would appear — but `BookRepository.List`/`ListRecent`'s `EXISTS (SELECT 1 FROM
copies ...)` filter (bookshelf-backend) means a copy-less book never surfaces in the catalog at
all. And `password-reset-magic-link.spec.ts` caught a real case-sensitivity bug in
`forgotPassword`: the magic-link JWT embedded a lowercased email as identifier, but
`resetPassword`'s token path fed that straight into a case-sensitive `FindByEmail`, silently
breaking the reset link for any account with uppercase characters in its email. A mocked `/api/books`
or `/api/auth/reset-password` response would have made both specs pass while shipping both bugs —
the mock only reflects what the test author _assumes_ the backend does, not what it actually does.

**On performance**: mocking does cut real cost — a real login pays bcrypt's deliberate ~150ms
hash-compare plus a DB round trip every time, and mocked specs also sidestep backend-side
rate-limiter/DB-state flake sources entirely.

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
webServers report healthy. It:

1. `POST`s directly to `http://localhost:8000/auth/setup` to create the first admin account
   (`E2E_ADMIN_EMAIL`/`E2E_ADMIN_PASSWORD`/`E2E_ADMIN_NAME`, in `src/test-users.ts` — kept out of
   `auth.setup.ts` itself since Playwright disallows importing one test file from another) — the
   one `auth.go` endpoint that issues a working account without an email-OTP round trip. Tolerates
   a 403 ("setup already complete") so re-runs against a `reuseExistingServer` backend in local dev
   (where the DB isn't wiped) stay idempotent.
2. Logs in through the UI and saves the resulting browser state (localStorage JWT) to
   `.auth/admin.json`. Specs that use admin auth only as a precondition (not testing the login form
   itself) load this via `test.use({ storageState: ".auth/admin.json" })` — see
   `book-cover-fallback.spec.ts` and `monthly-digest-jobs.spec.ts`. This skips the bcrypt
   round-trip (≈150ms + DB) on every test that would otherwise call `login()`. The file is
   regenerated on every suite run, so it never goes stale.

**Choosing between `storageState` and `login()`:**

| Scenario | Use |
| --- | --- |
| Spec navigates as admin, doesn't test the login form | `test.use({ storageState: ".auth/admin.json" })` + `page.goto(target)` directly |
| Spec tests the login UI itself (`login.spec.ts`, `password-reset-magic-link.spec.ts`) | `login()` from `auth-helpers.ts` — no `storageState` |
| Spec registers its own per-test user and logs in as them | `registerTestUser()` + `login()` — no admin storageState |

For specs in the first category, start the test with `page.goto(target)` instead of calling
`login()` — the storageState puts the admin JWT in localStorage before the page loads.
If the test needs to read the token from localStorage (e.g. to pass it as an `Authorization`
header), add `await page.goto("/catalog")` first so the evaluate happens on the app's origin
(storageState starts the page at `about:blank`).

A spec that registers its own one-off test user should use `E2E_TEST_USER_PASSWORD` from
`test-users.ts` for it, instead of inventing a fresh password literal — that way anyone poking at
a leftover test user later doesn't need to go dig through spec files to find its password. Only
fall back to a spec-local literal when the test itself needs a second, distinct password (e.g.
`password-reset-magic-link.spec.ts`'s `newPassword`, which has to differ from the original to
prove the reset actually took effect).

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

CI runs with `workers: 2` (overriding `nxE2EPreset`'s default of 1). The suite's rate limiters
are bumped in the e2e backend env to well above what two parallel workers can exhaust
(`REGISTER_RATE_LIMIT_BURST: 200`, `LOGIN_RATE_LIMIT_ATTEMPTS: 200`), and each spec is isolated
enough (own registered user, or the shared admin storageState) to run in parallel safely.
