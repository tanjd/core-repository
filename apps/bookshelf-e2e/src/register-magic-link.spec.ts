import { test, expect } from "@playwright/test";
import { startRegistration } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// The reported bug, reproduced literally: start signup in one place, open
// the emailed link somewhere else. `page.goto(debug_verify_link)` on a page
// that never visited /register carries no form state — same as reading the
// email on your phone after starting on a laptop — so if the link needed
// Name/Password back, this spec would land on a form instead of the catalog.
test("the emailed link finishes signup and signs you in, with no form to refill", async ({
  page,
  request,
}, testInfo) => {
  const email = `register-link-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}@example.com`;

  const { debug_verify_link } = await startRegistration(request, {
    name: "Register Link Tester",
    email,
    password: E2E_TEST_USER_PASSWORD,
  });
  expect(debug_verify_link).toContain("/register?verifyToken=");

  await page.goto(debug_verify_link);

  await expect(page).toHaveURL("/catalog");
  // Already signed in — no login round trip, and nothing was retyped.
  await expect(
    page.getByRole("button", { name: "Verify and create account" }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(() => localStorage.getItem("bookshelf_token")),
  ).toBeTruthy();
});

test("an already-used link reports the failure instead of showing a blank form", async ({
  page,
  request,
}, testInfo) => {
  const email = `register-link-used-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}@example.com`;

  const { debug_verify_link } = await startRegistration(request, {
    name: "Used Link Tester",
    email,
    password: E2E_TEST_USER_PASSWORD,
  });

  // First click consumes the code and creates the account.
  await page.goto(debug_verify_link);
  await expect(page).toHaveURL("/catalog");

  // Second click (e.g. the link tapped again later) has nothing left to
  // verify. The old behavior — silently landing on an empty Name/Password
  // form — is exactly what this asserts against.
  await page.goto(debug_verify_link);
  await expect(page.getByText("Link didn't work")).toBeVisible();
  await expect(page.getByLabel("Password", { exact: true })).toHaveCount(0);
});
