import { test, expect } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the Instagram/Facebook-style nav redesign (see
// apps/bookshelf/CLAUDE.md's "Mobile-first UI" section): the mobile bottom
// tab bar's centered "Share" popover (Scan ISBN / Search), the Wishlist tab,
// and the Profile-menu popover (Profile link + Logout) shared by both the
// mobile header and the desktop nav.
//
// Runs only on the "chromium" project and switches viewport mid-test
// (rather than also running on "Mobile Chrome") so covering both layouts
// costs one registerTestUser call, not two: the mobile/desktop split here is
// driven entirely by Tailwind's `md:` breakpoint (a CSS media query on
// viewport width), not by touch input or user-agent, so a plain viewport
// resize exercises the same code path "Mobile Chrome" would. This also
// keeps this spec from adding to the shared IP-wide /auth/register rate
// limit (5/10min, middleware.ClientIP in auth.go) on both projects — that
// budget is already close to used up across import-export-books.spec.ts and
// password-reset-magic-link.spec.ts's own registrations (each running on
// both projects), see auth-helpers.ts.
test("primary nav: desktop links, mobile tab bar/Share popover, and Profile menu with Logout", async ({
  page,
  request,
  isMobile,
}) => {
  test.skip(isMobile, "covers both breakpoints via viewport resize instead");
  const email = `nav-redesign-${Date.now()}@example.com`;
  await registerTestUser(
    request,
    email,
    E2E_TEST_USER_PASSWORD,
    "Nav Redesign Tester",
  );
  await login(page, email, E2E_TEST_USER_PASSWORD);

  // Desktop nav, at this project's default (wide) viewport.
  await expect(page.getByRole("link", { name: "Catalog" })).toBeVisible();
  await expect(page.getByRole("link", { name: "My Books" })).toBeVisible();
  await expect(page.getByRole("link", { name: "My Requests" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Wishlist" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: /^Notifications/ }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Profile" }).click();
  await expect(page.getByRole("link", { name: "Profile" })).toHaveAttribute(
    "href",
    "/profile",
  );
  await expect(page.getByRole("button", { name: "Logout" })).toBeVisible();
  await page.keyboard.press("Escape");

  // Resize below Tailwind's `md` breakpoint (768px) to switch into the
  // mobile bottom tab bar + header layout.
  await page.setViewportSize({ width: 390, height: 844 });

  await expect(page.getByRole("link", { name: "Catalog" })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Books", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Requests", exact: true }),
  ).toBeVisible();
  const wishlistTab = page.getByRole("link", { name: "Wishlist" });
  await expect(wishlistTab).toBeVisible();
  await expect(wishlistTab).toHaveAttribute("href", "/wishlist");

  await page
    .getByRole("button", { name: "Share a book — scan or search" })
    .click();
  const scanLink = page.getByRole("link", { name: "Scan ISBN" });
  const searchLink = page.getByRole("link", { name: "Search" });
  await expect(scanLink).toBeVisible();
  await expect(scanLink).toHaveAttribute("href", "/share/scan");
  await expect(searchLink).toBeVisible();
  await expect(searchLink).toHaveAttribute("href", "/share");
  await searchLink.click();
  await expect(page).toHaveURL("/share");

  await expect(
    page.getByRole("button", { name: /^Notifications/ }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Profile menu" }).click();
  await expect(page.getByRole("link", { name: "Profile" })).toHaveAttribute(
    "href",
    "/profile",
  );
  await page.getByRole("button", { name: "Logout" }).click();

  await expect(page).toHaveURL("/login");
  expect(
    await page.evaluate(() => localStorage.getItem("bookshelf_token")),
  ).toBeNull();
});
