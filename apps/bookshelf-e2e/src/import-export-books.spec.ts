import { test, expect } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers apps/bookshelf-backend's export/import round trip (GET
// /copies/mine/export, POST /copies/mine/import{,/preview}) and the My Books
// import UI (apps/bookshelf/src/app/my-books/page.tsx) end-to-end against
// the real backend. An imported file is untrusted input by design (it may
// come from a different bookshelf instance, or be hand-edited/corrupted in
// transit), so this exercises both "a genuine export re-imports cleanly,
// deduping against the existing catalog rather than duplicating it" and "a
// corrupted file fails safely without touching the catalog" — a mocked API
// response would let either of those regress silently.
//
// Both scenarios share one registerTestUser call (rather than one each):
// POST /auth/register carries its own IP-wide rate limit (5/10min,
// middleware.ClientIP in auth.go), shared across this whole e2e run's
// single localhost IP — registering a fresh user per scenario would burn
// through that budget alongside password-reset-magic-link.spec.ts's own
// registration.
test("exporting then re-importing a book matches it to the existing catalog entry; a corrupted file fails safely", async ({
  page,
  request,
}, testInfo) => {
  const email = `import-export-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}@example.com`;
  await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
  await login(page, email, E2E_TEST_USER_PASSWORD);

  const token = await page.evaluate(() =>
    localStorage.getItem("bookshelf_token"),
  );

  const title = `E2E Import Round Trip ${Date.now()}`;
  const isbn = `978${Date.now().toString().slice(-10)}`;

  const bookResponse = await page.request.post("/api/books", {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, author: "E2E Author", isbn },
  });
  expect(bookResponse.ok()).toBeTruthy();
  const book = await bookResponse.json();

  const copyResponse = await page.request.post("/api/copies", {
    headers: { Authorization: `Bearer ${token}` },
    data: { book_id: book.id, condition: "good" },
  });
  expect(copyResponse.ok()).toBeTruthy();

  const exportResponse = await page.request.get(
    "/api/copies/mine/export?format=json",
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(exportResponse.ok()).toBeTruthy();
  const exportContent = await exportResponse.text();
  expect(exportContent).toContain(isbn);

  await test.step("re-importing the export matches the existing book instead of duplicating it", async () => {
    await page.goto("/my-books");
    await page.getByRole("button", { name: "Import" }).click();
    await expect(
      page.getByRole("heading", { name: "Import Books" }),
    ).toBeVisible();

    await page.locator("#import-file-input").setInputFiles({
      name: "my-books.json",
      mimeType: "application/json",
      buffer: Buffer.from(exportContent),
    });

    // The re-imported row for our ISBN should match the book we just
    // created (findExistingBook's ISBN fallback), not create a duplicate.
    await expect(page.getByText("Matched", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Will import: 1 matched to your existing catalog"),
    ).toBeVisible();

    await page.getByRole("button", { name: "Import 1 book" }).click();
    await expect(page.getByRole("button", { name: "Done" })).toBeVisible();
    await page.getByRole("button", { name: "Done" }).click();

    const copiesResponse = await page.request.get("/api/copies/mine", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const copies: { id: number; book_id: number }[] =
      await copiesResponse.json();
    const matchingCopies = copies.filter((c) => c.book_id === book.id);
    expect(matchingCopies.length).toBe(2);
  });

  await test.step("a corrupted import file fails safely without touching the catalog", async () => {
    const before = await page.request.get("/api/copies/mine", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const beforeCount = (await before.json()).length;

    await page.goto("/my-books");
    await page.getByRole("button", { name: "Import" }).click();
    await expect(
      page.getByRole("heading", { name: "Import Books" }),
    ).toBeVisible();

    await page.locator("#import-file-input").setInputFiles({
      name: "corrupted.json",
      mimeType: "application/json",
      buffer: Buffer.from('[{"title": "Broken"'),
    });

    await expect(page.getByText(/invalid json/i)).toBeVisible();

    const after = await page.request.get("/api/copies/mine", {
      headers: { Authorization: `Bearer ${token}` },
    });
    const afterCount = (await after.json()).length;
    expect(afterCount).toBe(beforeCount);
  });
});
