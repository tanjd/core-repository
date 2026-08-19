import { test, expect } from "@playwright/test";

// TODO(tech debt): this suite runs `next dev` with no `bookshelf-backend`
// running (see playwright.config.ts), so the test below mocks
// `POST /api/auth/login` via page.route rather than exercising the real Go
// backend — it verifies frontend routing/session-persistence behavior on a
// successful login, not the actual request/response contract with the
// backend (a shape drift there, e.g. renaming `token`, wouldn't be caught
// here). Login→catalog is currently the *only* core path with e2e coverage;
// other core flows (book request/loan, wishlist fulfillment, admin
// approval) have none. Closing this gap for real means running this suite
// against a live backend (e.g. via docker-compose) instead of more route
// mocking — added now as a floor to catch login-path regressions, not as a
// substitute for that.
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
  await page.route("**/api/auth/login", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        token: "test-token",
        user: {
          id: 1,
          name: "Test User",
          email: "test@example.com",
          phone: "",
          verified: true,
          phone_verified: false,
          suspended: false,
          pending_approval: false,
          role: "user",
          created_at: new Date().toISOString(),
          google_books_key_configured: false,
          email_notifications_enabled: true,
        },
      }),
    });
  });

  await page.goto("/login");
  await page.getByLabel("Email").fill("test@example.com");
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page).toHaveURL("/catalog");
  expect(
    await page.evaluate(() => localStorage.getItem("bookshelf_token")),
  ).toBe("test-token");
});
