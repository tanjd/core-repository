import { test as setup, type APIRequestContext } from "@playwright/test";
import {
  E2E_ADMIN_EMAIL,
  E2E_ADMIN_NAME,
  E2E_ADMIN_PASSWORD,
} from "./test-users";

// Every app-router route with a static (non-dynamic) path. Requested once,
// serially, below — see warmRoutes' comment for why.
const APP_ROUTES = [
  "/",
  "/about",
  "/admin",
  "/admin/announcements",
  "/admin/backups",
  "/admin/dashboard",
  "/admin/invites",
  "/admin/jobs",
  "/admin/metadata",
  "/admin/profile",
  "/admin/settings",
  "/admin/users",
  "/catalog",
  "/changelog",
  "/forgot-password",
  "/login",
  "/loans",
  "/my-books",
  "/notifications",
  "/profile",
  "/register",
  "/share",
  "/share/scan",
  "/unsubscribe",
  "/wishlist",
];

// Requests every route once, serially, before the parallel browser projects
// start. Works around a Next.js 16.2.12 production-server bug: a route's
// client reference manifest is loaded and cached lazily on first request,
// and two requests for the same not-yet-warm route landing at once can race
// that load — one throws `InvariantError: The client reference manifest for
// route "..." does not exist`, and since this app has no Pages Router,
// Next's own fallback error page then fails too (`ENOENT ... pages/500.html`),
// surfacing to the test as a bare "Internal Server Error" page instead of
// the real route. Hit in practice on /profile: three spec files
// (invite-code-registration, monthly-digest-unsubscribe,
// monthly-digest-preference) all navigate there, and with `workers: 2` two
// of them landed on the same never-yet-requested route within milliseconds
// of each other at suite start. Requesting each route once here, before any
// browser project's tests run, means every route's manifest is already
// cached by the time real (parallel) traffic begins, so the race can't
// happen. https://github.com/vercel/next.js/issues/93862 is the closest
// upstream report — revisit this workaround if a Next.js upgrade fixes it.
async function warmRoutes(request: APIRequestContext) {
  for (const route of APP_ROUTES) {
    await request.get(route);
  }
}

// POSTs straight to the backend (bypassing the frontend's /api proxy, which
// isn't needed for seeding) to create the first admin account via
// /auth/setup — the one auth.go endpoint that issues a usable account
// without an email-OTP round trip. Tolerates "setup already complete" (403)
// so this stays idempotent against a `reuseExistingServer` backend in local
// dev, where the e2e DB isn't wiped between runs.
//
// After seeding, logs in through the UI and saves the resulting browser
// state (localStorage JWT + cookies) to .auth/admin.json. Specs that need
// admin auth but aren't testing the login flow itself load this via
// `test.use({ storageState: ".auth/admin.json" })` to skip the bcrypt
// round-trip — see book-cover-fallback.spec.ts and monthly-digest-jobs.spec.ts.
// The file is regenerated on every run (this setup always executes before any
// browser project), so it never goes stale.
setup("seed admin account", async ({ request, page }) => {
  await warmRoutes(request);

  const response = await request.post("http://localhost:8000/auth/setup", {
    data: {
      name: E2E_ADMIN_NAME,
      email: E2E_ADMIN_EMAIL,
      password: E2E_ADMIN_PASSWORD,
    },
  });

  if (!response.ok() && response.status() !== 403) {
    throw new Error(
      `failed to seed e2e admin account: ${response.status()} ${await response.text()}`,
    );
  }

  await page.goto("/login");
  await page.getByLabel("Email").fill(E2E_ADMIN_EMAIL);
  await page.getByLabel("Password", { exact: true }).fill(E2E_ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.waitForURL("/catalog");
  await page.context().storageState({ path: ".auth/admin.json" });
});
