import { join } from "node:path";
import { defineConfig, devices } from "@playwright/test";
import { nxE2EPreset } from "@nx/playwright/preset";
import { workspaceRoot } from "@nx/devkit";

// For CI, you may want to set BASE_URL to the deployed application.
const baseURL = process.env["BASE_URL"] || "http://localhost:3000";

// Test-only secret — never used outside this ephemeral, wiped-per-run e2e
// database, so there's nothing to protect by keeping it out of source.
const E2E_JWT_SECRET = "e2e-test-secret-do-not-use-in-production";

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  ...nxE2EPreset(__filename, { testDir: "./src" }),
  use: {
    baseURL,
    trace: "on-first-retry",
  },
  /* Two real servers, not mocks: the backend on :8000 (matching the
   * frontend's default BACKEND_URL, so no extra env wiring is needed) and
   * a production `next build && next start` on :3000. `auth.setup.ts` seeds
   * an admin account once both are up — see apps/bookshelf-e2e/CLAUDE.md for
   * the full rationale.
   *
   * The frontend deliberately runs a production build, not `next dev`:
   * dev's on-demand route compilation raced hard navigations (stale bundle)
   * and sometimes aborted them outright (net::ERR_ABORTED) — the dominant
   * source of flakiness in this suite, see apps/bookshelf-e2e/CLAUDE.md's
   * "Frontend runs a production build" section for the history. A
   * production build has no on-demand compilation, so that whole class of
   * flake doesn't happen — it costs a build up front (see this entry's
   * `timeout`) in exchange for not racing every navigation in every spec
   * run after.
   *
   * Both commands invoke the underlying tool directly (`go run`, `next
   * build`/`next start`) rather than going through `nx run
   * bookshelf-backend:serve` / `nx run bookshelf:serve`: Nx's continuous-task
   * executor forks its own `run-executor.js` helper to launch the real
   * process — when Playwright tree-kills the process it spawned on
   * teardown, that helper (and whatever it launched) can be left orphaned
   * instead of dying with it. Calling the tool directly makes it the
   * process Playwright owns, so teardown reliably kills the whole tree. */
  webServer: [
    {
      // DB_PATH is wiped before every run so each e2e run starts from a
      // clean, migrated database — see db.Open/runMigrations in
      // internal/db/db.go, which apply migrations on open.
      command:
        "rm -f data/e2e.db data/e2e.db-shm data/e2e.db-wal && go run ./cmd/server",
      url: "http://localhost:8000/health",
      reuseExistingServer: !process.env.CI,
      cwd: join(workspaceRoot, "apps/bookshelf-backend"),
      env: {
        PORT: "8000",
        DB_PATH: "./data/e2e.db",
        ENV: "dev",
        JWT_SECRET: E2E_JWT_SECRET,
        CORS_ORIGINS: "http://localhost:3000",
        FRONTEND_ORIGIN: "http://localhost:3000",
        // registerLimiter's default burst (20, internal/handlers/auth.go)
        // is sized for real community usage, not this suite: every spec
        // file registers its own account, all against one shared server,
        // with several starting within the same second at suite launch —
        // routinely enough to exhaust the default burst by ordinary
        // parallel-worker timing alone, well before any real signup volume.
        // See loan-request-flow.spec.ts's header comment for how this
        // surfaced.
        REGISTER_RATE_LIMIT_BURST: "200",
        // Explicitly blank, not just omitted: godotenv only fills in vars
        // that aren't already set, so a dev's real apps/bookshelf-backend/.env
        // (if one exists, e.g. real SMTP credentials for local testing)
        // would otherwise leak into this run and send real email for any
        // spec that exercises an email-sending flow (forgot-password,
        // registration, etc.).
        SMTP_HOST: "",
      },
    },
    {
      command:
        "pnpm exec next build apps/bookshelf --webpack && pnpm exec next start apps/bookshelf --port 3000",
      url: "http://localhost:3000",
      reuseExistingServer: !process.env.CI,
      cwd: workspaceRoot,
      // Playwright's webServer default timeout (60s) covers next dev's
      // near-instant boot but not a production build first — bump it so a
      // cold `next build` doesn't get killed mid-build on a slower CI runner.
      timeout: 180_000,
    },
  ],
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      // Mobile-resolution coverage — Pixel 7's viewport (412x839) is close
      // to the median real-world mobile viewport. Uses Chrome (its
      // `defaultBrowserType`), not WebKit, so it runs on the same
      // `chromium`-only browser binary CLAUDE.md has devs install — no
      // extra `playwright install` target needed.
      name: "Mobile Chrome",
      use: { ...devices["Pixel 7"] },
      dependencies: ["setup"],
    },
  ],
});
