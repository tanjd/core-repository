// Standalone script (not a *.spec.ts, so Playwright's testMatch never picks
// it up) that regenerates the four catalog screenshots LandingPage.tsx
// embeds (apps/bookshelf/public/screenshots/catalog-{desktop,mobile}{,-dark}.png).
// Re-run this whenever a catalog/nav UI change makes those screenshots stale.
//
// Expects both servers already running against a disposable DB (reuse
// playwright.config.ts's webServer commands, but point DB_PATH somewhere
// throwaway so seeding here doesn't touch e2e.db or a real dev DB):
//
//   cd apps/bookshelf-backend && DB_PATH=./data/screenshot-seed.db PORT=8000 \
//     ENV=dev JWT_SECRET=screenshot-gen-secret-do-not-use-in-production \
//     CORS_ORIGINS=http://localhost:3000 FRONTEND_ORIGIN=http://localhost:3000 \
//     SMTP_HOST= go run ./cmd/server
//   pnpm exec next build apps/bookshelf --webpack && pnpm exec next start apps/bookshelf --port 3000
//
// Then from the repo root: pnpm exec tsx apps/bookshelf-e2e/src/tools/generate-landing-screenshots.ts
import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import {
  E2E_ADMIN_EMAIL,
  E2E_ADMIN_NAME,
  E2E_ADMIN_PASSWORD,
} from "../test-users";

const BACKEND = "http://localhost:8000";
const FRONTEND = "http://localhost:3000";
const OUT_DIR = join(__dirname, "../../../bookshelf/public/screenshots");

// A mix of real ISBNs (real Open Library cover art, for visual variety) and
// two cover-less entries (BookCoverFallback's generated gradient, since not
// every community-shared book has metadata). Capped at 10 — copies.go
// enforces a per-user max of 10 shared copies — rather than BookshelfRow's
// full limit of 12 (apps/bookshelf/src/app/catalog/page.tsx).
const SEED_BOOKS: { title: string; author: string; isbn?: string }[] = [
  { title: "Clean Code", author: "Robert C. Martin", isbn: "9780132350884" },
  {
    title: "The Pragmatic Programmer",
    author: "David Thomas",
    isbn: "9780135957059",
  },
  { title: "Dune", author: "Frank Herbert", isbn: "9780441172719" },
  { title: "The Hobbit", author: "J.R.R. Tolkien", isbn: "9780547928227" },
  { title: "1984", author: "George Orwell", isbn: "9780451524935" },
  {
    title: "To Kill a Mockingbird",
    author: "Harper Lee",
    isbn: "9780061120084",
  },
  { title: "Atomic Habits", author: "James Clear", isbn: "9780735211292" },
  {
    title: "Project Hail Mary",
    author: "Andy Weir",
    isbn: "9780593135204",
  },
  { title: "Neighbourhood Trivia Night Rules", author: "J. Smith" },
  { title: "Church Potluck Recipe Book", author: "Community Kitchen" },
];

async function jsonFetch(url: string, init?: RequestInit) {
  const res = await fetch(url, {
    ...init,
    headers: { "content-type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    throw new Error(
      `${init?.method ?? "GET"} ${url} -> ${res.status} ${await res.text()}`,
    );
  }
  return res.json();
}

async function seed() {
  const setupRes = await fetch(`${BACKEND}/auth/setup`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      name: E2E_ADMIN_NAME,
      email: E2E_ADMIN_EMAIL,
      password: E2E_ADMIN_PASSWORD,
    }),
  });
  if (!setupRes.ok && setupRes.status !== 403) {
    throw new Error(
      `auth/setup failed: ${setupRes.status} ${await setupRes.text()}`,
    );
  }

  // Books are seeded under a second, throwaway user rather than the admin
  // account we log in as for the screenshot — otherwise every card in the
  // shot would carry BookCard's "Yours" badge, which reads wrong for a
  // landing-page screenshot meant to depict books the community shared.
  const seedEmail = `landing-screenshots-seed-${Date.now()}@example.com`;
  const seedPassword = "LandingScreenshotSeed1";
  const { debug_code } = await jsonFetch(
    `${BACKEND}/auth/register/send-email-otp`,
    {
      method: "POST",
      body: JSON.stringify({
        name: "Community Seed",
        email: seedEmail,
        password: seedPassword,
      }),
    },
  );
  // verify-email-otp creates the account outright — there's no separate
  // /auth/register call any more.
  await jsonFetch(`${BACKEND}/auth/register/verify-email-otp`, {
    method: "POST",
    body: JSON.stringify({ email: seedEmail, code: debug_code }),
  });
  const { token } = await jsonFetch(`${BACKEND}/auth/login`, {
    method: "POST",
    body: JSON.stringify({ email: seedEmail, password: seedPassword }),
  });
  const auth = { Authorization: `Bearer ${token}` };

  for (const seedBook of SEED_BOOKS) {
    const coverUrl = seedBook.isbn
      ? `https://covers.openlibrary.org/b/isbn/${seedBook.isbn}-L.jpg`
      : undefined;
    const book = await jsonFetch(`${BACKEND}/books`, {
      method: "POST",
      headers: auth,
      body: JSON.stringify({
        title: seedBook.title,
        author: seedBook.author,
        isbn: seedBook.isbn,
        cover_url: coverUrl,
      }),
    });
    await jsonFetch(`${BACKEND}/copies`, {
      method: "POST",
      headers: auth,
      body: JSON.stringify({ book_id: book.id, condition: "good" }),
    });
  }
}

type Shot = {
  name: string;
  width: number;
  height: number;
  deviceScaleFactor: number;
};

const SHOTS: Shot[] = [
  { name: "desktop", width: 1440, height: 960, deviceScaleFactor: 1.25 },
  { name: "mobile", width: 390, height: 664, deviceScaleFactor: 2 },
];

async function captureScreenshots() {
  mkdirSync(OUT_DIR, { recursive: true });
  const browser = await chromium.launch();

  for (const theme of ["light", "dark"] as const) {
    for (const shot of SHOTS) {
      const context = await browser.newContext({
        viewport: { width: shot.width, height: shot.height },
        deviceScaleFactor: shot.deviceScaleFactor,
        baseURL: FRONTEND,
      });
      const page = await context.newPage();

      await page.goto("/login");
      await page.getByLabel("Email").fill(E2E_ADMIN_EMAIL);
      await page
        .getByLabel("Password", { exact: true })
        .fill(E2E_ADMIN_PASSWORD);
      await page.getByRole("button", { name: "Sign in" }).click();
      await page.waitForURL("**/catalog");

      // next-themes reads localStorage("theme") on mount; setting it after
      // login and reloading is simpler than driving the ThemeToggle UI.
      await page.evaluate((t) => localStorage.setItem("theme", t), theme);
      await page.reload();
      // "networkidle" is banned repo-wide (playwright/no-networkidle) since
      // it's unreliable in general — here specifically, waiting on a
      // concrete catalog card is also just the more direct signal that the
      // themed re-render actually finished before the shot is taken.
      await page.locator('a[href^="/catalog/"]').first().waitFor();

      // File names follow LandingPage.tsx's convention: the "-dark" suffix
      // is the screenshot taken *in* dark mode (shown to light-mode
      // visitors as a peek at the other theme, and vice versa).
      const suffix = theme === "dark" ? "-dark" : "";
      const path = join(OUT_DIR, `catalog-${shot.name}${suffix}.png`);
      await page.screenshot({ path });
      console.log(`wrote ${path}`);

      await context.close();
    }
  }

  await browser.close();
}

async function main() {
  await seed();
  await captureScreenshots();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
