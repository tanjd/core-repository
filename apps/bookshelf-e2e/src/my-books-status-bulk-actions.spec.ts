import {
  test,
  expect,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

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

// Covers the My Books page additions from the "copy status badges / overdue
// nudges / bulk actions" plan: inline per-copy status (pending count, "on
// loan to X due Y"), and bulk select → pause lending / delete, including the
// backend's existing loaned/requested guard surfacing as a partial-success
// toast rather than a hard failure.
//
// Same setup pattern as loan-request-flow.spec.ts: real backend, one owner
// and one borrower account shared across this file's tests.

const BACKEND_URL = "http://localhost:8000";

async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<{ token: string }> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email, password },
  });
  expect(res.ok(), `login failed: ${await res.text()}`).toBeTruthy();
  return res.json();
}

async function createBookAndCopy(
  request: APIRequestContext,
  token: string,
  title: string,
): Promise<{ bookId: number; copyId: number }> {
  const bookRes = await request.post(`${BACKEND_URL}/books`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, author: "E2E Author" },
  });
  expect(
    bookRes.ok(),
    `create book failed: ${await bookRes.text()}`,
  ).toBeTruthy();
  const book = await bookRes.json();

  const copyRes = await request.post(`${BACKEND_URL}/copies`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { book_id: book.id, condition: "good" },
  });
  expect(
    copyRes.ok(),
    `create copy failed: ${await copyRes.text()}`,
  ).toBeTruthy();
  const copy = await copyRes.json();
  return { bookId: book.id as number, copyId: copy.id as number };
}

function uniqueEmail(label: string, testInfo: { project: { name: string } }) {
  return `${label}-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

test.describe("my books: status badges and bulk actions", () => {
  let ownerEmail: string;
  let ownerToken: string;
  let borrowerEmail: string;

  test.beforeAll(async ({ playwright }, testInfo) => {
    const setup = await playwright.request.newContext();
    ownerEmail = uniqueEmail("mybooks-owner", testInfo);
    borrowerEmail = uniqueEmail("mybooks-borrower", testInfo);

    await registerTestUser(setup, ownerEmail, E2E_TEST_USER_PASSWORD, "Owner");
    await registerTestUser(
      setup,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
      "Borrower",
    );

    ownerToken = (await apiLogin(setup, ownerEmail, E2E_TEST_USER_PASSWORD))
      .token;

    await setup.dispose();
  });

  test("a pending request shows an inline pending-count badge on My Books", async ({
    page,
    request,
  }) => {
    const bookTitle = `E2E Pending Badge ${Date.now()}`;
    const { bookId } = await createBookAndCopy(request, ownerToken, bookTitle);

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/my-books");
    // This is the first test in the file (see the describe-level comment on
    // shared owner/borrower accounts), so this owner's shelf has exactly one
    // requested copy at this point — a page-wide check is unambiguous, aside
    // from the desktop table and mobile card both rendering this title (only
    // one is CSS-hidden per apps/bookshelf/CLAUDE.md's responsive pattern —
    // see LendingTab.tsx), hence the visible-only filter.
    await expect(
      page.getByText(bookTitle).filter({ visible: true }),
    ).toBeVisible();
    // exact: the page-summary line above also contains the substring "1
    // pending" (as part of "1 pending request"), so a non-exact match is a
    // strict-mode violation — this is checking the per-copy badge alone.
    // Visible-only filter — see the comment above on the desktop/mobile
    // dual-render pattern.
    await expect(
      page.getByText("1 pending", { exact: true }).filter({ visible: true }),
    ).toBeVisible();
  });

  test("bulk pause and bulk delete act on selected copies", async ({
    page,
    request,
  }) => {
    const titleA = `E2E Bulk Pause A ${Date.now()}`;
    const titleB = `E2E Bulk Pause B ${Date.now()}`;
    const [a, b] = await Promise.all([
      createBookAndCopy(request, ownerToken, titleA),
      createBookAndCopy(request, ownerToken, titleB),
    ]);

    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/my-books");
    // Wait for the list to render before looking for the "Select" toggle —
    // it (and the checkboxes) don't exist until the copies have loaded.
    await expect(
      page.getByText(titleA).filter({ visible: true }),
    ).toBeVisible();

    await enterMobileSelectMode(page);
    await page
      .getByRole("checkbox", { name: `Select ${titleA}` })
      .filter({ visible: true })
      .check();
    await page
      .getByRole("checkbox", { name: `Select ${titleB}` })
      .filter({ visible: true })
      .check();
    // Visible-only filter — the desktop and mobile "select all" toolbars
    // both render this same "N selected" label, one CSS-hidden per
    // apps/bookshelf/CLAUDE.md's responsive pattern.
    await expect(
      page.getByText("2 selected").filter({ visible: true }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Pause lending" }).click();
    await page.getByRole("button", { name: "Pause 2 copies" }).click();
    await expect(page.getByText("2 copies paused")).toBeVisible();

    const copyRes = await request.get(`${BACKEND_URL}/copies/mine`, {
      headers: { Authorization: `Bearer ${ownerToken}` },
    });
    const copies: { id: number; status: string }[] = await copyRes.json();
    expect(copies.find((c) => c.id === a.copyId)?.status).toBe("unavailable");
    expect(copies.find((c) => c.id === b.copyId)?.status).toBe("unavailable");

    // Bulk delete both copies now that they're no longer loaned/requested.
    // Selection (and mobileSelectMode) reset after the pause action above.
    await enterMobileSelectMode(page);
    await page
      .getByRole("checkbox", { name: `Select ${titleA}` })
      .filter({ visible: true })
      .check();
    await page
      .getByRole("checkbox", { name: `Select ${titleB}` })
      .filter({ visible: true })
      .check();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Remove copies" }).click();
    await expect(page.getByText("2 copies removed")).toBeVisible();
    await expect(page.getByText(titleA)).toHaveCount(0);
    await expect(page.getByText(titleB)).toHaveCount(0);
  });

  test("bulk delete skips a requested copy and reports the partial result", async ({
    page,
    request,
  }) => {
    const requestedTitle = `E2E Bulk Skip Requested ${Date.now()}`;
    const availableTitle = `E2E Bulk Skip Available ${Date.now()}`;
    const { bookId: requestedBookId } = await createBookAndCopy(
      request,
      ownerToken,
      requestedTitle,
    );
    await createBookAndCopy(request, ownerToken, availableTitle);

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${requestedBookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/my-books");
    // Wait for the list to render before looking for the "Select" toggle —
    // it (and the checkboxes) don't exist until the copies have loaded.
    await expect(
      page.getByText(availableTitle).filter({ visible: true }),
    ).toBeVisible();

    await enterMobileSelectMode(page);
    await page
      .getByRole("checkbox", { name: `Select ${requestedTitle}` })
      .filter({ visible: true })
      .check();
    await page
      .getByRole("checkbox", { name: `Select ${availableTitle}` })
      .filter({ visible: true })
      .check();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("button", { name: "Remove copies" }).click();

    await expect(
      page.getByText("1 removed, 1 skipped (currently on loan or requested)"),
    ).toBeVisible();
    await expect(page.getByText(availableTitle)).toHaveCount(0);
    // Visible-only filter — see the comment on the equivalent assertion above.
    await expect(
      page.getByText(requestedTitle).filter({ visible: true }),
    ).toBeVisible();
  });

  test("clicking a book from My Books breadcrumbs back to My Books, not Catalog", async ({
    page,
    request,
  }) => {
    const bookTitle = `E2E Breadcrumb Origin ${Date.now()}`;
    await createBookAndCopy(request, ownerToken, bookTitle);

    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/my-books");
    // The cover image link's accessible name also starts with "E2E" (its
    // alt text), so this needs exact:true to avoid a strict-mode violation.
    await page.getByRole("link", { name: bookTitle, exact: true }).click();

    await expect(
      page
        .getByRole("navigation", { name: "Breadcrumb" })
        .getByRole("link", { name: "Back to My Books" }),
    ).toBeVisible();
    await page
      .getByRole("navigation", { name: "Breadcrumb" })
      .getByRole("link", { name: "Back to My Books" })
      .click();
    await expect(page).toHaveURL(/\/my-books$/);
  });
});
