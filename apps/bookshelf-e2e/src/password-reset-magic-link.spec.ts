import { test, expect } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

test("password reset via magic link requires no code entry", async ({
  page,
  request,
}, testInfo) => {
  const email = `magic-link-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}@example.com`;
  // Standard test-user password, kept distinct from newPassword below so the
  // reset flow's before/after can be told apart.
  const originalPassword = E2E_TEST_USER_PASSWORD;
  const newPassword = "ResetViaLinkPassw0rd9";

  await registerTestUser(request, email, originalPassword, "Magic Link Tester");

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

  await page.getByLabel("New password", { exact: true }).fill(newPassword);
  await page.getByLabel("Confirm new password").fill(newPassword);
  await page.getByRole("button", { name: "Reset password" }).click();
  await expect(page).toHaveURL("/login");

  // The new password actually works — the old one no longer would.
  await login(page, email, newPassword);
});
