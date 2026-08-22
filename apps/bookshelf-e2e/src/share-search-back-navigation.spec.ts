import { test, expect } from "@playwright/test";
import { E2E_TEST_USER_PASSWORD } from "./test-users";
import { login, registerTestUser } from "./auth-helpers";

// Mocked-API tier (see this app's CLAUDE.md's real-vs-mocked table): this spec is purely about
// frontend state persistence across a step change in MetadataSearchStep (apps/bookshelf/src/app/
// share/components/MetadataSearchStep.tsx) — not about what the real metadata providers return —
// so the exact result set and the search request count need to be deterministic, which mocking
// gives for free and a live Open Library/Google Books call wouldn't.
//
// Regression coverage for a real bug: SharePage (apps/bookshelf/src/app/share/page.tsx) used to
// render "search"/"confirm"/"manual" as mutually exclusive early `return`s, so picking a search
// result fully unmounted <MetadataSearchStep>, discarding its query/results state; clicking
// "Back to search" then remounted a brand-new instance with nothing in it. Fixed by keeping all
// three steps mounted simultaneously, toggled via a `hidden` class instead of unmounting.
//
// Registers its own one-off account rather than logging in as the shared E2E_ADMIN_EMAIL (the
// /share page's auth guard only requires a token, no admin role) — the shared admin's
// /auth/login budget (5 attempts/15min, internal/handlers/auth.go) is already spent by
// login.spec.ts and book-cover-fallback.spec.ts across both Playwright projects; a third spec
// logging in as it would push a single full suite run over that limit.
test("search results and query survive navigating back from the confirm step", async ({
  page,
  request,
}) => {
  const email = `share-back-nav-${Date.now()}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
  await login(page, email, E2E_TEST_USER_PASSWORD);

  let searchRequestCount = 0;
  await page.route("**/api/books/metadata/search**", async (route) => {
    searchRequestCount++;
    await route.fulfill({
      json: [
        {
          source: "google_books",
          title: "Go in Action",
          author: "William Kennedy",
          isbn: "9781617291769",
          cover_url: "",
          description: "A great book about Go",
          publisher: "Manning",
          published_date: "2015",
          page_count: 300,
          language: "en",
          ol_key: "",
          google_books_id: "abc123",
        },
      ],
    });
  });

  await page.goto("/share");
  await page
    .getByPlaceholder("Search by title, author, ISBN…")
    .fill("Go in Action");

  const resultButton = page.getByRole("button", { name: /Go in Action/ });
  await expect(resultButton).toBeVisible();
  expect(searchRequestCount).toBe(1);

  await resultButton.click();
  await expect(
    page.getByRole("heading", { name: "Confirm & share" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Back to search" }).click();

  // Same result still visible, no second network round trip — proves the
  // search step's own component state (query + cached results) survived
  // rather than being torn down and rebuilt from scratch.
  await expect(resultButton).toBeVisible();
  await expect(
    page.getByPlaceholder("Search by title, author, ISBN…"),
  ).toHaveValue("Go in Action");
  expect(searchRequestCount).toBe(1);
});
