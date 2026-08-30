import { test as setup } from "@playwright/test";
import {
  E2E_ADMIN_EMAIL,
  E2E_ADMIN_NAME,
  E2E_ADMIN_PASSWORD,
} from "./test-users";

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
