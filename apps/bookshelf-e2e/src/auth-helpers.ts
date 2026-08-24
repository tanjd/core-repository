import { type APIRequestContext, type Page, expect } from "@playwright/test";

// Shared UI login flow — used both by specs that log in as a means to an
// end (book-cover-fallback.spec.ts, the reset-then-login check in
// password-reset-magic-link.spec.ts) and by login.spec.ts itself, which
// asserts this exact sequence as the behavior under test. Kept in a plain
// module, not *.spec.ts, since Playwright disallows importing one test file
// from another (same reasoning as test-users.ts).
export async function login(page: Page, email: string, password: string) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL("/catalog");
}

// Registers a fresh user directly against the backend (bypassing the
// registration UI wizard — same direct-POST pattern as auth.setup.ts), so a
// spec that needs its own account doesn't collide with the shared e2e admin
// account other specs depend on — including /auth/login's per-email rate
// limit (5 attempts/15min, internal/handlers/auth.go), which the shared
// admin account's budget is already close to across every spec that logs in
// as it.
export async function registerTestUser(
  request: APIRequestContext,
  email: string,
  password: string,
  name = "E2E Test User",
) {
  // Two calls, not three: send-email-otp holds the whole form server-side,
  // and verify-email-otp creates the account outright.
  const sendOtp = await startRegistration(request, { name, email, password });

  // verify-email-otp shares registerLimiter (auth.go), an IP-wide 20-request
  // burst refilling only 1/30s, across every spec in this run — with ~13
  // spec files x 2 browser projects all registering at roughly the same
  // suite-start instant, that burst is routinely exhausted by nothing more
  // than ordinary parallel-worker timing, not a real problem with any one
  // spec. A bounded retry with backoff long enough to matter against that
  // refill rate absorbs the transient 429 instead of failing the whole spec
  // on scheduling luck; a non-429 failure (or persistence past every retry)
  // still surfaces immediately; the assert below runs every attempt's
  // response through the same message either way.
  let verifyOtp = await request.post(
    "http://localhost:8000/auth/register/verify-email-otp",
    { data: { email, code: sendOtp.debug_code } },
  );
  for (let attempt = 0; verifyOtp.status() === 429 && attempt < 3; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 10_000 * (attempt + 1)));
    verifyOtp = await request.post(
      "http://localhost:8000/auth/register/verify-email-otp",
      { data: { email, code: sendOtp.debug_code } },
    );
  }
  // Body in the message, not just ok(): this endpoint is IP-rate-limited
  // (registerLimiter), and a bare `false` gives no hint that a run
  // registering many users tripped it rather than the flow being broken.
  expect(
    verifyOtp.ok(),
    `verify-email-otp failed (${verifyOtp.status()}): ${await verifyOtp.text()}`,
  ).toBeTruthy();
}

/**
 * Submits the registration form's details step and returns the dev-only
 * debug fields — `debug_verify_link` is the exact magic-link URL the
 * verification email would carry, which no SMTP is configured to deliver in
 * this run (see playwright.config.ts).
 */
export async function startRegistration(
  request: APIRequestContext,
  data: { name: string; email: string; password: string; phone?: string },
): Promise<{ debug_code: string; debug_verify_link: string }> {
  const res = await request.post(
    "http://localhost:8000/auth/register/send-email-otp",
    { data },
  );
  expect(res.ok()).toBeTruthy();
  return res.json();
}
