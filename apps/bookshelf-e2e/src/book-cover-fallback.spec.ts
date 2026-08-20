import { test, expect } from "@playwright/test";
import { E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD } from "./test-users";

// Covers the generated book-cover fallback (BookCover/BookCoverFallback in
// apps/bookshelf/src/components/BookCover.tsx) end-to-end against the real
// backend: a book created with no cover_url should render a presentable
// generated SVG cover — not a broken image or a bare icon — everywhere the
// catalog surfaces it (the "Recently Added" spine shelf, the catalog grid,
// and the book detail page).
test("a book with no cover renders a generated cover fallback across the catalog", async ({
  page,
}) => {
  await page.goto("/login");
  await page.getByLabel("Email").fill(E2E_ADMIN_EMAIL);
  await page.getByLabel("Password").fill(E2E_ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL("/catalog");

  const token = await page.evaluate(() =>
    localStorage.getItem("bookshelf_token"),
  );
  const title = `E2E Cover Fallback ${Date.now()}`;
  const author = "E2E Author";

  const createResponse = await page.request.post("/api/books", {
    headers: { Authorization: `Bearer ${token}` },
    data: { title, author },
  });
  expect(createResponse.ok()).toBeTruthy();
  const book = await createResponse.json();

  // A book with no copies never surfaces in the catalog (see
  // BookRepository.List/ListRecent's `EXISTS (SELECT 1 FROM copies ...)`
  // filter in bookshelf-backend) — same createBook + createCopy pairing the
  // "Share a Book" page does (src/app/share/page.tsx).
  const createCopyResponse = await page.request.post("/api/copies", {
    headers: { Authorization: `Bearer ${token}` },
    data: { book_id: book.id },
  });
  expect(createCopyResponse.ok()).toBeTruthy();

  const coverPlaceholder = page.getByRole("img", {
    name: `Cover placeholder for ${title}, by ${author}`,
  });

  // "Recently Added" spine shelf (BookshelfRow/BookSpine), only shown when
  // the catalog isn't being searched.
  await page.goto("/catalog");
  await expect(
    page.getByRole("heading", { name: "Book Catalog" }),
  ).toBeVisible();
  await expect(coverPlaceholder.first()).toBeVisible();

  // Catalog grid (BookCard), isolated via search.
  await page.getByPlaceholder("Search by title, author…").fill(title);
  await expect(coverPlaceholder.first()).toBeVisible();

  // Book detail page.
  await page
    .getByRole("link", { name: new RegExp(title) })
    .first()
    .click();
  await expect(page).toHaveURL(/\/catalog\/\d+/);
  await expect(page.getByRole("heading", { name: title })).toBeVisible();
  await expect(coverPlaceholder).toBeVisible();
});
