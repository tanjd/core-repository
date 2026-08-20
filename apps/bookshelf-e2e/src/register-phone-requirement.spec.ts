import { test, expect } from "@playwright/test";

// Exercises docs/specs/bookshelf-dynamic-phone-requirement.md: the
// registration page elevates the phone field to required, and blocks
// submission without one, only when the community's
// `verification_requires_phone` admin setting is on — otherwise phone stays
// optional exactly as it does today.
//
// Mocked, not real-backend: the setting this page reacts to is a single
// global admin toggle (`GET /auth/registration-requirements`), and this
// suite runs the same spec across two Playwright projects (chromium, Mobile
// Chrome) in parallel against one shared backend/DB — flipping that global
// toggle from both projects at once races. What's under test here is purely
// how the registration page renders a given `require_phone` response, not
// whether the backend computes that response correctly (that's covered by
// the Go handler test `TestRegistrationRequirements` in
// apps/bookshelf-backend), so mocking the one endpoint sidesteps the race
// without losing coverage — see apps/bookshelf-e2e/CLAUDE.md's "Real backend
// vs. mocked API" section.
async function mockRequirePhone(
  page: import("@playwright/test").Page,
  requirePhone: boolean,
) {
  await page.route("**/api/auth/registration-requirements", (route) =>
    route.fulfill({ json: { require_phone: requirePhone } }),
  );
}

test.describe("registration phone requirement", () => {
  test("stays optional when the community does not require it", async ({
    page,
  }) => {
    await mockRequirePhone(page, false);

    await page.goto("/register");
    const phoneInput = page.getByLabel("Phone number");
    await expect(phoneInput).toBeEnabled();
    await expect(page.getByText("(optional)")).toBeVisible();

    await page.getByLabel("Name").fill("Optional Phone Tester");
    await page
      .getByLabel("Email")
      .fill(`optional-phone-${Date.now()}@example.com`);
    await page
      .getByLabel("Password", { exact: true })
      .fill("OptionalPassw0rd1");
    await page.getByLabel("Confirm password").fill("OptionalPassw0rd1");
    // Phone left blank on purpose.
    await page.getByRole("button", { name: "Continue" }).click();

    // No blocking error — the flow proceeds to email verification.
    await expect(
      page.getByText("This community requires a phone number to register"),
    ).toHaveCount(0);
    await expect(page.getByText("Verify your email")).toBeVisible();
  });

  test("is required and blocks submission when the community requires it", async ({
    page,
  }) => {
    await mockRequirePhone(page, true);

    await page.goto("/register");
    const phoneInput = page.getByLabel("Phone number");
    await expect(phoneInput).toBeEnabled();
    await expect(page.getByText("(optional)")).toHaveCount(0);
    await expect(
      page.getByText(
        "This community requires a verified phone number to borrow books.",
      ),
    ).toBeVisible();

    await page.getByLabel("Name").fill("Required Phone Tester");
    await page
      .getByLabel("Email")
      .fill(`required-phone-${Date.now()}@example.com`);
    await page
      .getByLabel("Password", { exact: true })
      .fill("RequiredPassw0rd1");
    await page.getByLabel("Confirm password").fill("RequiredPassw0rd1");
    // Phone left blank — submit should be blocked.
    await page.getByRole("button", { name: "Continue" }).click();

    await expect(
      page.getByText("This community requires a phone number to register"),
    ).toBeVisible();
    await expect(page.getByText("Verify your email")).toHaveCount(0);

    // Filling it in unblocks the flow.
    await phoneInput.fill("9123 4567");
    await page.getByRole("button", { name: "Continue" }).click();
    await expect(page.getByText("Verify your email")).toBeVisible();
  });
});
