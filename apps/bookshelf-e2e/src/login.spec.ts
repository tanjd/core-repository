import { test, expect } from "@playwright/test";

test("login page renders the sign-in form", async ({ page }) => {
  await page.goto("/login");

  await expect(
    page.getByText("Enter your email and password to access Bookshelf"),
  ).toBeVisible();
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});
