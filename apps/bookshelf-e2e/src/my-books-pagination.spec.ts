import { test, expect, type Page } from "@playwright/test";

// Bulk-select checkboxes are opt-in on mobile (a "Select" toggle in the
// toolbar) but always visible on desktop — see apps/bookshelf/CLAUDE.md's
// "Bulk-select checkboxes are opt-in on mobile" note. The toggle only
// renders in the mobile-only toolbar block, so it's absent (not just
// hidden) on the chromium project — hence the isVisible check rather than
// clicking unconditionally.
async function enterMobileSelectMode(page: Page) {
  const selectToggle = page.getByRole("button", {
    name: "Select",
    exact: true,
  });
  if (await selectToggle.isVisible()) {
    await selectToggle.click();
  }
}

// Mocked API, not real backend — deliberately, per apps/bookshelf-e2e/CLAUDE.md's
// "seeding 50 books to test pagination" example of when mocking is the right
// call: producing 25 real books/copies through the real backend just to
// exercise a client-side pagination cutoff would be slow and adds nothing
// over asserting how the page renders a given /copies/mine response. Uses
// the shared admin storageState purely as a logged-in precondition — this
// spec isn't testing auth or the backend contract, only the My Books page's
// client-side grouping/pagination of whatever GET /copies/mine returns.

test.use({ storageState: ".auth/admin.json" });

const BOOK_COUNT = 25;
const PAGE_SIZE = 20;

function mockCopies() {
  return Array.from({ length: BOOK_COUNT }, (_, i) => {
    const n = String(i + 1).padStart(2, "0");
    return {
      id: i + 1,
      book_id: i + 1,
      owner_id: 1,
      condition: "good" as const,
      notes: "",
      status: "available" as const,
      book: {
        id: i + 1,
        title: `E2E Pagination Book ${n}`,
        author: "E2E Author",
        isbn: "",
        ol_key: "",
        cover_url: "",
        description: "",
      },
    };
  });
}

test.describe("my books: pagination", () => {
  test.beforeEach(async ({ page }) => {
    await page.route("**/api/copies/mine", (route) =>
      route.fulfill({ json: mockCopies() }),
    );
    await page.route("**/api/loan-requests?copy_id=*", (route) =>
      route.fulfill({ json: [] }),
    );
  });

  test("paginates books beyond the first page and pages between them", async ({
    page,
  }) => {
    await page.goto("/my-books");

    // Visible-only filter: the desktop table and mobile card list both
    // render every group (one CSS-hidden per breakpoint via apps/bookshelf's
    // responsive convention), so an unfiltered getByText matches two nodes.
    await expect(
      page.getByText("E2E Pagination Book 01").filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page
        .getByText(`E2E Pagination Book ${PAGE_SIZE}`, { exact: true })
        .filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page.getByText("E2E Pagination Book 21").filter({ visible: true }),
    ).toHaveCount(0);

    await page.getByRole("button", { name: "Page 2" }).click();

    await expect(
      page.getByText("E2E Pagination Book 01").filter({ visible: true }),
    ).toHaveCount(0);
    await expect(
      page.getByText("E2E Pagination Book 21").filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page
        .getByText(`E2E Pagination Book ${BOOK_COUNT}`)
        .filter({ visible: true }),
    ).toBeVisible();
  });

  test("searching resets pagination back to page 1", async ({ page }) => {
    await page.goto("/my-books");

    await page.getByRole("button", { name: "Page 2" }).click();
    await expect(
      page.getByText("E2E Pagination Book 21").filter({ visible: true }),
    ).toBeVisible();

    await page.getByPlaceholder("Search by title or author…").fill("Book 01");

    await expect(
      page.getByText("E2E Pagination Book 01").filter({ visible: true }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Page 2" })).toHaveCount(0);
  });

  test("bulk-deleting the rest of page 2 falls back to page 1 instead of rendering blank", async ({
    page,
  }) => {
    // Regression test: nothing previously clamped `page` after the list
    // shrank out from under it — bulk-deleting every copy on page 2 (ids
    // 21-25, leaving 20 total = exactly one page) used to leave `page`
    // stuck at 2, rendering a table/card list with zero rows and no
    // "no results" messaging, even though page 1 still had 20 books.
    // Single handler (registered after the describe-level beforeEach's
    // "**/api/copies/mine" route, so it takes priority for every match)
    // covering both GET .../copies/mine and DELETE .../copies/:id, so the
    // bulk delete below actually shrinks what the next GET returns.
    let copies = mockCopies();
    await page.route("**/api/copies/**", (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      if (method === "GET" && url.pathname.endsWith("/copies/mine")) {
        return route.fulfill({ json: copies });
      }
      if (method === "DELETE") {
        const id = Number(url.pathname.split("/").pop());
        copies = copies.filter((c) => c.id !== id);
        return route.fulfill({ status: 204, body: "" });
      }
      return route.continue();
    });

    await page.goto("/my-books");
    await page.getByRole("button", { name: "Page 2" }).click();
    await expect(
      page.getByText("E2E Pagination Book 21").filter({ visible: true }),
    ).toBeVisible();

    // Select just the 5 books on page 2 (ids 21-25) — a plain "select all"
    // would span every filtered book across both pages per the "Selection
    // spans all filtered books" comment in page.tsx, deleting everything
    // instead of leaving page 1's 20 books to fall back onto.
    await enterMobileSelectMode(page);
    for (let n = 21; n <= 25; n++) {
      await page
        .getByRole("checkbox", { name: `Select E2E Pagination Book ${n}` })
        .filter({ visible: true })
        .check();
    }
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Remove copies" }).click();

    await expect(
      page.getByText("E2E Pagination Book 01").filter({ visible: true }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Page 2" })).toHaveCount(0);
  });
});
