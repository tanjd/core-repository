import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the "To Read" feature end-to-end against the real backend: tapping
// the bookmark toggle on a catalog card, the book appearing under Catalog's
// "To Read" segment, and un-saving it from there.
//
// Same setup style as book-recommendations.spec.ts: one user, registered
// once in beforeAll; one book created via the API. Skipped on Mobile Chrome
// for the same reason — neither surface here is viewport-dependent.

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
): Promise<number> {
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
  return book.id as number;
}

function uniqueEmail(label: string, testInfo: { project: { name: string } }) {
  return `${label}-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

test.describe("to read", () => {
  let memberEmail: string;
  let memberName: string;
  let bookTitle: string;

  test.beforeAll(async ({ playwright }, testInfo) => {
    if (testInfo.project.name === MOBILE_PROJECT) return;

    const setup = await playwright.request.newContext();
    memberEmail = uniqueEmail("to-read-member", testInfo);
    memberName = "To Read Member";

    await registerTestUser(
      setup,
      memberEmail,
      E2E_TEST_USER_PASSWORD,
      memberName,
    );
    const token = await apiLogin(setup, memberEmail, E2E_TEST_USER_PASSWORD);

    bookTitle = `AAA To Read ${Date.now()}`;
    await createBookAndCopy(setup, token, bookTitle);

    await setup.dispose();
  });

  test("tap-to-save on a catalog card, appears under the To Read segment, and un-saving", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );

    await login(page, memberEmail, E2E_TEST_USER_PASSWORD);

    // Scope the catalog to just this spec's book.
    await page.goto(`/catalog?q=${encodeURIComponent(bookTitle)}`);

    const card = page
      .locator('a[href^="/catalog/"]')
      .filter({ hasText: bookTitle });
    const saveButton = card.getByRole("button", {
      name: `Save ${bookTitle} to read later`,
    });
    await expect(saveButton).toBeVisible();
    await saveButton.click();

    // Optimistic update: flips immediately, without leaving the catalog.
    const savedButton = card.getByRole("button", {
      name: `Remove ${bookTitle} from your to-read list`,
    });
    await expect(savedButton).toBeVisible();
    await expect(page).toHaveURL(/\/catalog\?/);

    // The "To Read" segment shows the saved book.
    await page.goto("/catalog?view=to-read");
    await expect(page.getByText(bookTitle)).toBeVisible();

    // Un-save from the To Read segment — it disappears from the list.
    await page
      .locator('a[href^="/catalog/"]')
      .filter({ hasText: bookTitle })
      .getByRole("button", {
        name: `Remove ${bookTitle} from your to-read list`,
      })
      .click();
    await expect(page.getByText(bookTitle)).toBeHidden();

    // Back on the main catalog, the card reflects the removal too.
    await page.goto(`/catalog?q=${encodeURIComponent(bookTitle)}`);
    await expect(
      card.getByRole("button", { name: `Save ${bookTitle} to read later` }),
    ).toBeVisible();
  });
});
