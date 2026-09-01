import { test, expect } from "@playwright/test";

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

    await page
      .getByPlaceholder("Search your books by title, author…")
      .fill("Book 01");

    await expect(
      page.getByText("E2E Pagination Book 01").filter({ visible: true }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Page 2" })).toHaveCount(0);
  });
});
