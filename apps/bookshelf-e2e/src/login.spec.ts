import { test, expect } from "@playwright/test";
import { E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD } from "./test-users";

// Login→catalog is currently the only core path with e2e coverage; other
// core flows (book request/loan, wishlist fulfillment, admin approval) have
// none — see apps/bookshelf-e2e/CLAUDE.md for the plan to close that gap.
test("login page renders the sign-in form", async ({ page }) => {
  await page.goto("/login");

  await expect(
    page.getByText("Enter your email and password to access Bookshelf"),
  ).toBeVisible();
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});

test("successful login redirects to the catalog and persists the session", async ({
  page,
}) => {
  await page.goto("/login");
  await page.getByLabel("Email").fill(E2E_ADMIN_EMAIL);
  await page.getByLabel("Password").fill(E2E_ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL("/catalog");
  // A real JWT from the backend now, not a fixture — just assert it's set.
  expect(
    await page.evaluate(() => localStorage.getItem("bookshelf_token")),
  ).toBeTruthy();
});
