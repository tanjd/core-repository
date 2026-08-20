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
  const sendOtp = await request.post(
    "http://localhost:8000/auth/register/send-email-otp",
    { data: { email } },
  );
  expect(sendOtp.ok()).toBeTruthy();
  const { debug_code } = await sendOtp.json();

  const verifyOtp = await request.post(
    "http://localhost:8000/auth/register/verify-email-otp",
    { data: { email, code: debug_code } },
  );
  expect(verifyOtp.ok()).toBeTruthy();
  const { verification_token } = await verifyOtp.json();

  const register = await request.post("http://localhost:8000/auth/register", {
    data: {
      name,
      email,
      password,
      email_verification_token: verification_token,
    },
  });
  expect(register.ok()).toBeTruthy();
}
