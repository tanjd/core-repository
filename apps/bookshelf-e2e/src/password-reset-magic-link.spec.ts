import { test, expect } from "@playwright/test";

// Registers a fresh user directly against the backend (bypassing the
// registration UI wizard — same direct-POST pattern as auth.setup.ts) so
// this spec's own forgot-password/reset-password round trip doesn't collide
// with the shared e2e admin account other specs depend on.
async function registerTestUser(
  request: import("@playwright/test").APIRequestContext,
  email: string,
  password: string,
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
      name: "Magic Link Tester",
      email,
      password,
      email_verification_token: verification_token,
    },
  });
  expect(register.ok()).toBeTruthy();
}

test("password reset via magic link requires no code entry", async ({
  page,
  request,
}, testInfo) => {
  const email = `magic-link-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}@example.com`;
  const originalPassword = "OriginalPassw0rd1";
  const newPassword = "ResetViaLinkPassw0rd9";

  await registerTestUser(request, email, originalPassword);

  // Request a reset and pull the magic link straight from the dev-only
  // debug field — no SMTP configured for this e2e run (see
  // playwright.config.ts), so this is the same link the email would embed.
  const forgot = await request.post(
    "http://localhost:8000/auth/forgot-password",
    { data: { email } },
  );
  expect(forgot.ok()).toBeTruthy();
  const { debug_reset_link } = await forgot.json();
  expect(debug_reset_link).toContain("/forgot-password?resetToken=");

  // Follow the link exactly as a user clicking it from their email would.
  await page.goto(debug_reset_link);

  await expect(
    page.getByText("Enter a new password to complete your reset."),
  ).toBeVisible();
  // The magic-link path skips code entry entirely.
  await expect(page.getByLabel("Reset code")).toHaveCount(0);

  await page.getByLabel("New password").fill(newPassword);
  await page.getByLabel("Confirm new password").fill(newPassword);
  await page.getByRole("button", { name: "Reset password" }).click();

  await expect(page).toHaveURL("/login");

  // The new password actually works — the old one no longer would.
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(newPassword);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL("/catalog");
});
