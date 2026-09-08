import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers apps/bookshelf/docs/author-view-spec.md end-to-end against the real
// backend: tapping an author name on a catalog card lands on the per-author
// page listing every book by that exact author, and the Catalog page's
// Authors toggle lists distinct authors with a correct book count that also
// links through to the same per-author page.

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
  author: string,
): Promise<number> {
  const bookRes = await request.post(`${BACKEND_URL}/books`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, author },
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
  return book.id as number;
}

function uniqueEmail(label: string, testInfo: { project: { name: string } }) {
  return `${label}-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

test.describe("author view", () => {
  let memberEmail: string;
  let authorName: string;
  let firstTitle: string;
  let secondTitle: string;

  test.beforeAll(async ({ playwright }, testInfo) => {
    if (testInfo.project.name === MOBILE_PROJECT) return;

    const setup = await playwright.request.newContext();
    memberEmail = uniqueEmail("author-view-member", testInfo);
    await registerTestUser(setup, memberEmail, E2E_TEST_USER_PASSWORD);
    const token = await apiLogin(setup, memberEmail, E2E_TEST_USER_PASSWORD);

    const stamp = Date.now();
    // A unique author per run so this spec's count assertions aren't
    // sensitive to other specs' books sharing the default "E2E Author".
    authorName = `AAA Author View ${stamp}`;
    firstTitle = `Author View Book One ${stamp}`;
    secondTitle = `Author View Book Two ${stamp}`;

    await createBookAndCopy(setup, token, firstTitle, authorName);
    await createBookAndCopy(setup, token, secondTitle, authorName);

    await setup.dispose();
  });

  test("tapping an author name on a catalog card shows every book by that exact author", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — covers the tap-through link plus the per-author page",
    );

    await login(page, memberEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog?q=${encodeURIComponent(authorName)}`);

    await expect(page.getByText(firstTitle).first()).toBeVisible();

    await page.getByRole("button", { name: authorName }).first().click();
    await expect(page).toHaveURL(
      `/authors/${encodeURIComponent(authorName)}?from=${encodeURIComponent("/catalog")}`,
    );
    await expect(page.getByRole("heading", { name: authorName })).toBeVisible();
    await expect(page.getByText(firstTitle)).toBeVisible();
    await expect(page.getByText(secondTitle)).toBeVisible();
    await expect(page.getByText("2 books in the catalog")).toBeVisible();
  });

  test("the Authors toggle on Catalog lists this author with the right count and links through", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — covers the Authors index view",
    );

    await login(page, memberEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/catalog");

    await page.getByRole("button", { name: "Authors" }).click();
    await expect(page).toHaveURL(/\?view=authors$/);

    const authorRow = page.getByRole("link", { name: new RegExp(authorName) });
    await expect(authorRow).toBeVisible();
    await expect(authorRow.getByText("2 books")).toBeVisible();

    await authorRow.click();
    await expect(page).toHaveURL(
      `/authors/${encodeURIComponent(authorName)}?from=${encodeURIComponent("/catalog?view=authors")}`,
    );
    await expect(page.getByText(firstTitle)).toBeVisible();
    await expect(page.getByText(secondTitle)).toBeVisible();
  });
});
