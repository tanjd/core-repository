import { type Page, expect } from "@playwright/test";

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
