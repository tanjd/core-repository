import { test, expect } from "@playwright/test";

// The `verification_requires_phone` admin setting gates *borrowing*, not
// signing up — see apps/bookshelf/docs/magic-link-registration-spec.md. On
// the registration page it only changes the phone field's helper copy;
// submission goes through with or without a phone either way. (It used to
// block submission, and force a mocked SMS OTP step on anyone who filled it
// in — neither proved anything, since no SMS provider is configured.)
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
// vs. mocked API" section. Everything past the mock — the actual submission
// — still hits the real backend.
async function mockRequirePhone(
  page: import("@playwright/test").Page,
  requirePhone: boolean,
) {
  await page.route("**/api/auth/registration-requirements", (route) =>
    route.fulfill({ json: { require_phone: requirePhone } }),
  );
}

const BORROW_HINT =
  "You'll need a phone number on file to borrow books in this community";

async function fillDetails(
  page: import("@playwright/test").Page,
  name: string,
  email: string,
  password: string,
) {
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password").fill(password);
}

test.describe("registration phone requirement", () => {
  test("says nothing about phone when the community does not require one", async ({
    page,
  }, testInfo) => {
    await mockRequirePhone(page, false);
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;

    await page.goto("/register");
    await expect(page.getByLabel("Phone number")).toBeEnabled();
    await expect(page.getByText("(optional)")).toBeVisible();
    await expect(page.getByText(BORROW_HINT)).toHaveCount(0);

    await fillDetails(
      page,
      "Optional Phone Tester",
      `optional-phone-${suffix}@example.com`,
      "OptionalPassw0rd1",
    );
    // Phone left blank on purpose.
    await page.getByRole("button", { name: "Continue" }).click();

    await expect(page.getByText("Verify your email")).toBeVisible();
  });

  test("explains the borrowing requirement but still lets signup through without a phone", async ({
    page,
  }, testInfo) => {
    await mockRequirePhone(page, true);
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;

    await page.goto("/register");
    await expect(page.getByLabel("Phone number")).toBeEnabled();
    // Still optional at signup — the requirement is surfaced, not enforced.
    await expect(page.getByText("(optional)")).toBeVisible();
    await expect(page.getByText(BORROW_HINT)).toBeVisible();

    await fillDetails(
      page,
      "Required Phone Tester",
      `required-phone-${suffix}@example.com`,
      "RequiredPassw0rd1",
    );
    // Phone left blank — this used to block submission; it no longer does.
    await page.getByRole("button", { name: "Continue" }).click();

    await expect(page.getByText("Verify your email")).toBeVisible();
    await expect(
      page.getByText("This community requires a phone number to register"),
    ).toHaveCount(0);
  });

  test("a phone entered at signup goes straight through, with no OTP step", async ({
    page,
  }, testInfo) => {
    await mockRequirePhone(page, true);
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;

    await page.goto("/register");
    await fillDetails(
      page,
      "With Phone Tester",
      `with-phone-${suffix}@example.com`,
      "WithPhonePassw0rd1",
    );
    await page.getByLabel("Phone number").fill("9123 4567");
    await page.getByRole("button", { name: "Continue" }).click();

    // Email verification is the only step — the mocked phone OTP screen is
    // gone, so registration is one verification round trip regardless.
    await expect(page.getByText("Verify your email")).toBeVisible();
    await expect(page.getByText("Verify your phone")).toHaveCount(0);
  });
});
