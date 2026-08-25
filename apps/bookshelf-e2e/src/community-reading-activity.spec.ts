import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers Feature A from
// apps/bookshelf/docs/community-reading-activity-spec.md — the catalog's
// new "Most Borrowed" sort, and the book-detail page's stats row derived
// from completed-loan activity — end-to-end against the real backend.
//
// Same setup style as loan-request-flow.spec.ts: one owner + one borrower,
// registered once in beforeAll; the loan cycle needed to give a book a
// non-zero borrow count runs through the backend API (POST loan-request,
// PATCH accept) rather than the UI, so this spec only drives the browser
// for the two things it's actually asserting on — the sort ordering and
// the stats row. Skipped on Mobile Chrome for the same reason
// loan-request-flow.spec.ts is: neither surface is viewport-dependent.

const BACKEND_URL = "http://localhost:8000";
const MOBILE_PROJECT = "Mobile Chrome";

async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<string> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email, password },
  });
  expect(res.ok(), `login failed: ${await res.text()}`).toBeTruthy();
  const body = await res.json();
  return body.token as string;
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

async function completeLoan(
  request: APIRequestContext,
  ownerToken: string,
  borrowerToken: string,
  copyId: number,
) {
  const reqRes = await request.post(`${BACKEND_URL}/loan-requests`, {
    headers: { Authorization: `Bearer ${borrowerToken}` },
    data: { copy_id: copyId, message: "e2e" },
  });
  expect(
    reqRes.ok(),
    `create loan request failed: ${await reqRes.text()}`,
  ).toBeTruthy();
  const loanReq = await reqRes.json();

  const acceptRes = await request.patch(
    `${BACKEND_URL}/loan-requests/${loanReq.id}`,
    {
      headers: { Authorization: `Bearer ${ownerToken}` },
      data: { status: "accepted" },
    },
  );
  expect(
    acceptRes.ok(),
    `accept loan request failed: ${await acceptRes.text()}`,
  ).toBeTruthy();
}

function uniqueEmail(label: string, testInfo: { project: { name: string } }) {
  return `${label}-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

test.describe("community reading activity", () => {
  let ownerEmail: string;
  let ownerToken: string;
  let borrowerEmail: string;
  let borrowerToken: string;
  let borrowedBookTitle: string;
  let untouchedBookTitle: string;
  let borrowedBookId: number;

  test.beforeAll(async ({ playwright }, testInfo) => {
    if (testInfo.project.name === MOBILE_PROJECT) return;

    const setup = await playwright.request.newContext();
    ownerEmail = uniqueEmail("reading-activity-owner", testInfo);
    borrowerEmail = uniqueEmail("reading-activity-borrower", testInfo);

    await registerTestUser(setup, ownerEmail, E2E_TEST_USER_PASSWORD, "Owner");
    await registerTestUser(
      setup,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
      "Borrower",
    );

    ownerToken = await apiLogin(setup, ownerEmail, E2E_TEST_USER_PASSWORD);
    borrowerToken = await apiLogin(
      setup,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
    );

    // Two books with an intentionally alphabetical ordering that would
    // put the untouched book first under Title-A→Z. sort=popular has to
    // reverse that.
    const stamp = Date.now();
    borrowedBookTitle = `AAA Reading Activity Borrowed ${stamp}`;
    untouchedBookTitle = `AAA Reading Activity Untouched ${stamp}`;

    const borrowed = await createBookAndCopy(
      setup,
      ownerToken,
      borrowedBookTitle,
    );
    borrowedBookId = borrowed.bookId;
    await createBookAndCopy(setup, ownerToken, untouchedBookTitle);
    await completeLoan(setup, ownerToken, borrowerToken, borrowed.copyId);

    await setup.dispose();
  });

  test("book detail page surfaces a completed-loan count in a stats row", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${borrowedBookId}`);

    await expect(page.getByText(/borrowed 1 time/i)).toBeVisible();
  });

  test("catalog Most Borrowed sort ranks a borrowed book above an untouched one", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    // Search-scope the catalog to just this test's two books so the
    // ordering assertion isn't sensitive to whatever other books earlier
    // specs (or a reused local DB) may have left behind.
    await page.goto(`/catalog?q=AAA+Reading+Activity&sort=popular`);

    await expect(page.getByText(borrowedBookTitle)).toBeVisible();
    await expect(page.getByText(untouchedBookTitle)).toBeVisible();

    // DOM-order check rather than bounding-box y: the catalog is a grid,
    // so two adjacent results can sit on the same row (identical y) even
    // when the sort is correct. Each BookCard is wrapped in a
    // <Link href="/catalog/{id}">, so the anchor list is the render order.
    const cardTexts = await page
      .locator('a[href^="/catalog/"]')
      .filter({ hasText: /AAA Reading Activity/ })
      .allInnerTexts();
    const borrowedIdx = cardTexts.findIndex((t) =>
      t.includes(borrowedBookTitle),
    );
    const untouchedIdx = cardTexts.findIndex((t) =>
      t.includes(untouchedBookTitle),
    );
    expect(
      borrowedIdx,
      "borrowed card should be present",
    ).toBeGreaterThanOrEqual(0);
    expect(
      untouchedIdx,
      "untouched card should be present",
    ).toBeGreaterThanOrEqual(0);
    expect(borrowedIdx).toBeLessThan(untouchedIdx);
  });
});
