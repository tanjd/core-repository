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
setup("seed admin account", async ({ request }) => {
  const response = await request.post("http://localhost:8000/auth/setup", {
    data: {
      name: E2E_ADMIN_NAME,
      email: E2E_ADMIN_EMAIL,
      password: E2E_ADMIN_PASSWORD,
    },
  });

  if (response.ok() || response.status() === 403) {
    return;
  }

  throw new Error(
    `failed to seed e2e admin account: ${response.status()} ${await response.text()}`,
  );
});
