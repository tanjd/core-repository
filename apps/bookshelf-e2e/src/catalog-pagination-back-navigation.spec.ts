import { test, expect } from "@playwright/test";
import { E2E_TEST_USER_PASSWORD } from "./test-users";
import { login, registerTestUser } from "./auth-helpers";

// Mocked-API tier (see bookshelf-e2e's CLAUDE.md real-vs-mocked table):
// reproducing a second results page through the real backend would mean
// seeding 20+ books with copies, which this spec has no other reason to do
// — the thing under test is purely how CatalogPage persists page state
// across a navigation, not what the backend returns for a given page.
//
// Both tests below are regression coverage for the same underlying concern:
// catalog page state (page number, filters, sort) must survive round-trips
// through a book detail page, whether the user returns via the browser Back
// button or via the in-page breadcrumb.

const page1Books = Array.from({ length: 20 }, (_, i) => ({
  id: i + 1,
  title: `Page One Book ${i + 1}`,
  author: "Author One",
  isbn: "",
  ol_key: "",
  cover_url: "",
  description: "",
  available_copies: 1,
}));
const page2Book = {
  id: 21,
  title: "Page Two Book",
  author: "Author Two",
  isbn: "",
  ol_key: "",
  cover_url: "",
  description: "",
  copies: [],
  available_copies: 1,
};

async function setupCatalogMocks(page: import("@playwright/test").Page) {
  await page.route("**/api/books/recent**", async (route) => {
    await route.fulfill({ json: [] });
  });
  await page.route("**/api/books/21**", async (route) => {
    await route.fulfill({ json: page2Book });
  });
  // Registered after the broader "**/api/books/21**" route above so it
  // takes precedence (Playwright tries matching routes last-registered
  // first) — otherwise that route's catch-all would also answer the book
  // detail page's GET .../recommendations call with the Book JSON object
  // instead of an array, crashing RecommendedBy's render.
  await page.route("**/api/books/21/recommendations**", async (route) => {
    await route.fulfill({ json: [] });
  });
  await page.route("**/api/books*", async (route) => {
    const url = new URL(route.request().url());
    const requestedPage = Number(url.searchParams.get("page") ?? "1");
    const items = requestedPage >= 2 ? [page2Book] : page1Books;
    await route.fulfill({
      json: {
        items,
        total: page1Books.length + 1,
        page: requestedPage,
        page_size: 20,
        total_pages: 2,
      },
    });
  });
}

// Regression: CatalogPage kept `page`/`search`/`sort`/`availableOnly` in
// plain useState with no URL reflection, so navigating into a book's detail
// page and clicking the browser Back button remounted CatalogPage fresh —
// `useState(1)` re-initialized `page` to 1, silently dropping the user back
// to page 1. Fixed by syncing state into the URL query string (via
// router.replace) and hydrating it back out on mount.
test("catalog page number survives navigating into a book and back", async ({
  page,
  request,
}, testInfo) => {
  const email = `catalog-pagination-back-${Date.now()}-${testInfo.parallelIndex}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
  await setupCatalogMocks(page);

  await login(page, email, E2E_TEST_USER_PASSWORD);
  await expect(
    page.getByText("Page One Book 1", { exact: true }).first(),
  ).toBeVisible();

  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
  await expect(page).toHaveURL(/\?page=2$/);

  await page.getByRole("link", { name: /Page Two Book/ }).click();
  await expect(page).toHaveURL(/\/catalog\/21/);

  await page.goBack();
  await expect(page).toHaveURL(/\?page=2$/);
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
});

// Regression: the breadcrumb on the book detail page hardcoded href="/catalog",
// so clicking it always reset to page 1 regardless of which catalog page the
// user came from. Fixed by embedding the current catalog URL as a ?from= param
// in each BookCard link, then reading it back in the detail page breadcrumb.
test("breadcrumb returns to the catalog page the user came from", async ({
  page,
  request,
}, testInfo) => {
  const email = `catalog-breadcrumb-${Date.now()}-${testInfo.parallelIndex}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
  await setupCatalogMocks(page);

  await login(page, email, E2E_TEST_USER_PASSWORD);
  await expect(
    page.getByText("Page One Book 1", { exact: true }).first(),
  ).toBeVisible();

  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
  await expect(page).toHaveURL(/\?page=2$/);

  await page.getByRole("link", { name: /Page Two Book/ }).click();
  await expect(page).toHaveURL(/\/catalog\/21/);

  // Wait for the book to load so the breadcrumb is present and the 'from'
  // param has been read by the detail page's useEffect.
  await expect(
    page.getByRole("link", { name: "Back to catalog" }),
  ).toBeVisible();

  await page.getByRole("link", { name: "Back to catalog" }).click();
  await expect(page).toHaveURL(/\?page=2$/);
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
});

// Regression: navigating directly to /catalog?page=2 (e.g. via the breadcrumb
// or a bookmark) would briefly flash page 2 books and then snap back to
// /catalog (page 1). The root cause was that the debounce effect's
// mountedRef.current=true (set on the first invocation) was not reset before
// the React Strict Mode simulated remount, so the second invocation treated
// the mount as a user-triggered search/filter change and called
// updateUrl("","title",false,1), stripping the ?page param. Fixed by adding
// a [] reset effect that always runs before the debounce effect.
test("navigating directly to /catalog?page=2 stays on page 2", async ({
  page,
  request,
}, testInfo) => {
  const email = `catalog-direct-page-${Date.now()}-${testInfo.parallelIndex}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
  await setupCatalogMocks(page);

  await login(page, email, E2E_TEST_USER_PASSWORD);

  // Navigate directly to page 2 — simulates both a breadcrumb click landing
  // on this URL and a user typing/bookmarking it directly.
  await page.goto("/catalog?page=2");

  await expect(page).toHaveURL(/\?page=2$/);
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
});
