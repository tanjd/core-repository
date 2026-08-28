import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers apps/bookshelf/docs/book-recommendations-spec.md end-to-end
// against the real backend: tapping the recommend toggle on a catalog
// card, the "Most Recommended" sort, and the detail-page facepile —
// including removing a recommendation and confirming both surfaces revert.
//
// Same setup style as community-reading-activity.spec.ts: one user,
// registered once in beforeAll; two books with a deliberately alphabetical
// naming that would put the untouched book first under Title A→Z, so
// sort=recommended has to visibly reverse that. Skipped on Mobile Chrome
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

test.describe("book recommendations", () => {
  let memberEmail: string;
  let memberName: string;
  let recommendedBookTitle: string;
  let untouchedBookTitle: string;
  let recommendedBookId: number;

  test.beforeAll(async ({ playwright }, testInfo) => {
    if (testInfo.project.name === MOBILE_PROJECT) return;

    const setup = await playwright.request.newContext();
    memberEmail = uniqueEmail("recommendations-member", testInfo);
    memberName = "Recommendations Member";

    await registerTestUser(
      setup,
      memberEmail,
      E2E_TEST_USER_PASSWORD,
      memberName,
    );
    const token = await apiLogin(setup, memberEmail, E2E_TEST_USER_PASSWORD);

    const stamp = Date.now();
    recommendedBookTitle = `AAA Recommendations Recommended ${stamp}`;
    untouchedBookTitle = `AAA Recommendations Untouched ${stamp}`;

    recommendedBookId = await createBookAndCopy(
      setup,
      token,
      recommendedBookTitle,
    );
    await createBookAndCopy(setup, token, untouchedBookTitle);

    await setup.dispose();
  });

  test("tap-to-recommend on a catalog card, sort=recommended ordering, detail-page facepile, and un-recommending", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );

    await login(page, memberEmail, E2E_TEST_USER_PASSWORD);

    // Scope the catalog to just this spec's two books so the ordering
    // assertion below isn't sensitive to whatever else earlier specs (or a
    // reused local DB) left behind.
    await page.goto(`/catalog?q=${encodeURIComponent("AAA Recommendations")}`);

    const recommendedCard = page
      .locator('a[href^="/catalog/"]')
      .filter({ hasText: recommendedBookTitle });
    const recommendButton = recommendedCard.getByRole("button", {
      name: `Recommend ${recommendedBookTitle}`,
    });
    await expect(recommendButton).toBeVisible();
    await recommendButton.click();

    // Optimistic update: fills immediately, count goes 0 → 1, without
    // leaving the catalog. Scoped to inside the button itself (exact
    // match) so it can't ambiguously match the availability badge's
    // "1 available" text elsewhere on the same card.
    const filledButton = recommendedCard.getByRole("button", {
      name: `Remove your recommendation for ${recommendedBookTitle}`,
    });
    await expect(filledButton).toBeVisible();
    await expect(filledButton.getByText("1", { exact: true })).toBeVisible();
    await expect(page).toHaveURL(/\/catalog\?/);

    // sort=recommended should rank the just-recommended book above the
    // untouched one, reversing their alphabetical order.
    await page.goto(
      `/catalog?q=${encodeURIComponent("AAA Recommendations")}&sort=recommended`,
    );
    await expect(page.getByText(recommendedBookTitle)).toBeVisible();
    await expect(page.getByText(untouchedBookTitle)).toBeVisible();
    const cardTexts = await page
      .locator('a[href^="/catalog/"]')
      .filter({ hasText: /AAA Recommendations/ })
      .allInnerTexts();
    const recommendedIdx = cardTexts.findIndex((t) =>
      t.includes(recommendedBookTitle),
    );
    const untouchedIdx = cardTexts.findIndex((t) =>
      t.includes(untouchedBookTitle),
    );
    expect(recommendedIdx).toBeGreaterThanOrEqual(0);
    expect(untouchedIdx).toBeGreaterThanOrEqual(0);
    expect(recommendedIdx).toBeLessThan(untouchedIdx);

    // Detail page: same toggle state, and the facepile shows the recommender.
    await page.goto(`/catalog/${recommendedBookId}`);
    await expect(
      page.getByRole("button", {
        name: `Remove your recommendation for ${recommendedBookTitle}`,
      }),
    ).toBeVisible();
    await expect(page.getByText("Recommended by")).toBeVisible();

    // Un-recommend from the detail page — count and facepile both revert.
    await page
      .getByRole("button", {
        name: `Remove your recommendation for ${recommendedBookTitle}`,
      })
      .click();
    await expect(
      page.getByRole("button", {
        name: `Recommend ${recommendedBookTitle}`,
      }),
    ).toBeVisible();
    await expect(page.getByText("Recommended by")).toBeHidden();

    // Back on the catalog, the card reflects the removal too.
    await page.goto(`/catalog?q=${encodeURIComponent("AAA Recommendations")}`);
    const unfilledButton = recommendedCard.getByRole("button", {
      name: `Recommend ${recommendedBookTitle}`,
    });
    await expect(unfilledButton).toBeVisible();
    await expect(unfilledButton.getByText("1", { exact: true })).toBeHidden();
  });
});
