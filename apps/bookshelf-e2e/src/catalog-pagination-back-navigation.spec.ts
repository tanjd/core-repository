import { test, expect } from "@playwright/test";
import { E2E_TEST_USER_PASSWORD } from "./test-users";
import { login, registerTestUser } from "./auth-helpers";

// Mocked-API tier (see bookshelf-e2e's CLAUDE.md real-vs-mocked table):
// reproducing a second results page through the real backend would mean
// seeding 20+ books with copies, which this spec has no other reason to do
// — the thing under test is purely how CatalogPage persists page state
// across a navigation, not what the backend returns for a given page.
//
// Regression coverage for a real bug: CatalogPage (apps/bookshelf/src/app/
// catalog/page.tsx) kept `page`/`search`/`sort`/`availableOnly` in plain
// useState with no URL reflection, so navigating into a book's detail page
// and clicking the browser Back button remounted CatalogPage fresh —
// `useState(1)` re-initialized `page` to 1, silently dropping the user back
// to page 1 instead of the page they were on. Fixed by syncing that state
// into the URL query string (via router.replace) and hydrating it back out
// on mount.
test("catalog page number survives navigating into a book and back", async ({
  page,
  request,
}) => {
  const email = `catalog-pagination-${Date.now()}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);

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
  const page2Books = [
    {
      id: 21,
      title: "Page Two Book",
      author: "Author Two",
      isbn: "",
      ol_key: "",
      cover_url: "",
      description: "",
      copies: [],
      available_copies: 1,
    },
  ];

  await page.route("**/api/books/recent**", async (route) => {
    await route.fulfill({ json: [] });
  });
  await page.route("**/api/books/21", async (route) => {
    await route.fulfill({ json: page2Books[0] });
  });
  await page.route("**/api/books*", async (route) => {
    const url = new URL(route.request().url());
    const requestedPage = Number(url.searchParams.get("page") ?? "1");
    const items = requestedPage >= 2 ? page2Books : page1Books;
    await route.fulfill({
      json: {
        items,
        total: page1Books.length + page2Books.length,
        page: requestedPage,
        page_size: 20,
        total_pages: 2,
      },
    });
  });

  await login(page, email, E2E_TEST_USER_PASSWORD);
  await expect(
    page.getByText("Page One Book 1", { exact: true }).first(),
  ).toBeVisible();

  await page.getByRole("button", { name: "Next page" }).click();
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
  await expect(page).toHaveURL(/\?page=2$/);

  await page.getByRole("link", { name: /Page Two Book/ }).click();
  await expect(page).toHaveURL("/catalog/21");

  await page.goBack();
  await expect(page).toHaveURL(/\?page=2$/);
  await expect(page.getByText("Page Two Book").first()).toBeVisible();
});
