import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the loan-request UX fixes described in
// apps/bookshelf/docs/loan-request-ux-fixes-spec.md, end-to-end against the
// real backend — this is the first coverage this suite has for the
// borrow/loan/return flow (apps/bookshelf-e2e/CLAUDE.md's "Coverage" section
// previously called this out as untested). Also covers
// apps/bookshelf/docs/return-date-default-spec.md: every loan now always
// carries a return date (no more per-copy return_date_required toggle), and
// either party can amend it after acceptance via the "Edit return date"
// affordance (ReturnDateCell.tsx).
//
// All setup/verification calls hit the backend directly on :8000 (same
// pattern as auth-helpers.ts's registerTestUser), bypassing the frontend's
// /api proxy — these routes carry no "/api" prefix on the backend itself.
//
// Only two accounts (owner, borrower) are registered, once in beforeAll,
// and reused across every test below — including for the "a second
// member" scenario, which opens a second browser context logged in as the
// *same* borrower account rather than registering a third identity. That
// still exercises the thing under test (does a session that isn't the one
// which submitted the request get a working action on a "requested" copy,
// gated purely on copy status) without the cost of another account: this
// suite's shared IP-wide registerLimiter (verify-email-otp, auth.go) runs
// with very little headroom already — see primary-navigation.spec.ts's own
// comment on the same budget — and this file also runs only on the
// "chromium" project (test.skip in each test body, same pattern as that
// file) since none of these four fixes are viewport-dependent, so nothing
// "Mobile Chrome" would catch that chromium doesn't.

const BACKEND_URL = "http://localhost:8000";
const MOBILE_PROJECT = "Mobile Chrome";

async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<{ token: string; userId: number }> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email, password },
  });
  expect(res.ok(), `login failed: ${await res.text()}`).toBeTruthy();
  const body = await res.json();
  return { token: body.token as string, userId: body.user.id as number };
}

async function createBookAndCopy(
  request: APIRequestContext,
  token: string,
  opts: {
    title: string;
    autoApprove?: boolean;
  },
): Promise<{ bookId: number; copyId: number }> {
  const bookRes = await request.post(`${BACKEND_URL}/books`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { title: opts.title, author: "E2E Author" },
  });
  expect(
    bookRes.ok(),
    `create book failed: ${await bookRes.text()}`,
  ).toBeTruthy();
  const book = await bookRes.json();

  const copyRes = await request.post(`${BACKEND_URL}/copies`, {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      book_id: book.id,
      condition: "good",
      auto_approve: opts.autoApprove ?? false,
    },
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

test.describe("loan request flow", () => {
  // Shared across every test below — see the header comment on why.
  let ownerEmail: string;
  let ownerToken: string;
  let borrowerEmail: string;

  test.beforeAll(async ({ playwright }, testInfo) => {
    if (testInfo.project.name === MOBILE_PROJECT) return;

    const setup = await playwright.request.newContext();
    ownerEmail = uniqueEmail("loan-flow-owner", testInfo);
    borrowerEmail = uniqueEmail("loan-flow-borrower", testInfo);

    await registerTestUser(setup, ownerEmail, E2E_TEST_USER_PASSWORD, "Owner");
    await registerTestUser(
      setup,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
      "Borrower",
    );

    // Only the owner needs a token ahead of any UI login (to create books/
    // copies via the API below) — the borrower account only ever needs a
    // session, which each test establishes itself via the login UI.
    ownerToken = (await apiLogin(setup, ownerEmail, E2E_TEST_USER_PASSWORD))
      .token;

    await setup.dispose();
  });

  test("an auto-approve copy shows an instant-approval affordance and approves the request immediately", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { bookId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Auto-Approve ${Date.now()}`,
      autoApprove: true,
    });

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);

    await expect(page.getByText(/Instant approval/)).toBeVisible();

    // Auto-approve copies get a differentiated primary CTA ("Borrow
    // instantly") rather than the generic "Request to Borrow" — the
    // difference in behavior is surfaced on the button itself, not
    // buried in a badge.
    await page.getByRole("button", { name: "Borrow instantly" }).click();
    await expect(
      page.getByText(/auto-approves.*owner's contact info right away/i),
    ).toBeVisible();
    await page.getByRole("button", { name: "Send Request" }).click();

    await expect(
      page.getByText(
        "Request approved — check Loans for the owner's contact info",
      ),
    ).toBeVisible();

    await page.goto("/loans");
    // Scoped to the table: the same row also renders in a mobile card
    // (CSS-hidden, not removed, at this desktop-only project's viewport —
    // see apps/bookshelf/CLAUDE.md's "Cards over dense tables on narrow
    // screens"), so an unscoped getByText would be a strict-mode violation.
    await expect(
      page.getByRole("table").getByText("accepted", { exact: true }),
    ).toBeVisible();
  });

  test("a second session sees a way to act on a copy that's pending someone else's request, and is notified once it opens back up", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Requested Limbo ${Date.now()}`,
    });

    // First session submits the request that puts the copy into "requested".
    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    // A second, separate session (deliberately the same account — see the
    // header comment — the thing under test is the copy-status gating, not
    // who's logged in) must not hit a dead end.
    const contextB = await browser.newContext();
    const pageB = await contextB.newPage();
    await login(pageB, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await pageB.goto(`/catalog/${bookId}`);

    await expect(pageB.getByText("Requested", { exact: true })).toBeVisible();
    await expect(
      pageB.getByText(/already asked to borrow this/i),
    ).toBeVisible();
    await pageB.getByRole("button", { name: "Join Waitlist" }).click();
    await expect(
      pageB.getByRole("button", { name: "Leave Waitlist" }),
    ).toBeVisible();

    // Owner rejects the pending request — the copy should revert to
    // available and the waitlisted session (joined while the copy was
    // merely "requested", not yet "loaned") should get notified.
    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    expect(listRes.ok()).toBeTruthy();
    const requests: { id: number; status: string }[] = await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected the pending request to exist");

    const rejectRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "rejected" },
      },
    );
    expect(rejectRes.ok()).toBeTruthy();

    const bookRes = await request.get(`${BACKEND_URL}/books/${bookId}`, {
      headers: { Authorization: `Bearer ${ownerToken}` },
    });
    const book = await bookRes.json();
    const copy = book.copies.find((c: { id: number }) => c.id === copyId);
    expect(copy.status).toBe("available");

    await pageB.goto("/notifications");
    await expect(pageB.getByText("Copy now available")).toBeVisible();

    await contextB.close();
  });

  test("an overdue loan is flagged for both the borrower and the owner", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Overdue ${Date.now()}`,
    });

    // Borrower requests via the UI (rather than a raw API call) so this
    // test doesn't need its own apiLogin — the session from this login also
    // covers the later "view Loans" step below.
    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { id: number; status: string }[] = await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected a pending request to exist");

    const acceptRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "accepted" },
      },
    );
    expect(acceptRes.ok()).toBeTruthy();

    // return-date-default-spec.md's guardrail rejects a date before today, so
    // this can no longer backdate to yesterday to force "overdue" — today's
    // date still qualifies, since expected_return_date parses to midnight UTC
    // and the overdue check compares against the current (later-in-the-day)
    // timestamp.
    const today = new Date().toISOString().slice(0, 10);
    const dateRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}/expected-return-date`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { expected_return_date: today },
      },
    );
    expect(
      dateRes.ok(),
      `set return date failed: ${await dateRes.text()}`,
    ).toBeTruthy();

    await page.goto("/loans");
    // Scoped to the table — see the similar comment earlier in this file.
    await expect(
      page.getByRole("table").getByText("Overdue", { exact: true }),
    ).toBeVisible();

    const contextOwner = await browser.newContext();
    const ownerPage = await contextOwner.newPage();
    await login(ownerPage, ownerEmail, E2E_TEST_USER_PASSWORD);
    await ownerPage.goto("/my-books");
    await expect(ownerPage.getByText(/overdue since/i)).toBeVisible();

    await ownerPage.goto(`/my-books/${copyId}/requests`);
    // Scoped to the desktop table — the mobile card list renders the same
    // "Overdue" badge alongside it (CSS-hidden, not DOM-absent).
    await expect(
      ownerPage.getByRole("table").getByText("Overdue", { exact: true }),
    ).toBeVisible();

    await contextOwner.close();
  });

  test("the owner can override the borrower's proposed return date when accepting a request", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Counter Date ${Date.now()}`,
    });

    const proposedDate = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    const ownerProposedDate = new Date(Date.now() + 21 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.locator("#return-date").fill(proposedDate);
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const contextOwner = await browser.newContext();
    const ownerPage = await contextOwner.newPage();
    await login(ownerPage, ownerEmail, E2E_TEST_USER_PASSWORD);
    await ownerPage.goto(`/my-books/${copyId}/requests`);

    await ownerPage.getByRole("button", { name: "Accept" }).click();
    await expect(
      ownerPage.getByRole("heading", { name: "Accept Request" }),
    ).toBeVisible();
    await expect(ownerPage.locator("#accept-return-date")).toHaveValue(
      proposedDate,
    );

    await ownerPage.locator("#accept-return-date").fill(ownerProposedDate);
    await ownerPage.getByRole("button", { name: "Accept Request" }).click();
    // Scoped to the desktop table: the mobile card list renders the same
    // "accepted" text alongside it (CSS-hidden, not DOM-absent — see
    // apps/bookshelf/CLAUDE.md's "Cards over dense tables on narrow
    // screens"), so a bare page-wide getByText is a strict-mode violation.
    await expect(
      ownerPage.getByRole("table").getByText("accepted", { exact: true }),
    ).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { status: string; expected_return_date: string }[] =
      await listRes.json();
    const accepted = requests.find((r) => r.status === "accepted");
    if (!accepted) throw new Error("expected the request to be accepted");
    expect(accepted.expected_return_date.slice(0, 10)).toBe(ownerProposedDate);

    await contextOwner.close();
  });

  test("accepting a request reveals the owner's Telegram username and contact note to the borrower", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const bookTitle = `E2E Contact Extras ${Date.now()}`;
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: bookTitle,
    });

    const profileRes = await request.patch(`${BACKEND_URL}/auth/me`, {
      headers: { Authorization: `Bearer ${ownerToken}` },
      data: {
        telegram_username: "@e2e_owner",
        contact_note: "meet at the front desk on Saturdays",
      },
    });
    expect(profileRes.ok()).toBeTruthy();

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { id: number; status: string }[] = await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected a pending request to exist");

    const acceptRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "accepted" },
      },
    );
    expect(acceptRes.ok()).toBeTruthy();

    await page.goto("/loans");
    // Scoped to this test's own row: borrowerEmail is shared across every
    // test in this file (see the header comment), so by this 8th test
    // /loans' Borrowing tab already has other "accepted" rows from earlier
    // tests — a bare page-wide getByText("accepted") is a strict-mode
    // violation.
    const row = page.getByRole("row", { name: bookTitle });
    await expect(row.getByText("accepted", { exact: true })).toBeVisible();
    await row.getByText(bookTitle).click();
    // Scoped to the desktop table — expanded contact detail is rendered
    // twice (table expand-row + mobile card, per BorrowingTab.tsx).
    const table = page.getByRole("table");
    await expect(table.getByRole("link", { name: "@e2e_owner" })).toBeVisible();
    await expect(
      table.getByText("meet at the front desk on Saturdays"),
    ).toBeVisible();
  });

  test("the book detail page prioritises an auto-approve copy as the hero action and breadcrumbs back to the catalog", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    // Two copies for the same book: a plain "request" copy created
    // first, then an auto-approve copy created second — this proves
    // the hero action is picked by *ranking* (auto-approve wins),
    // not insertion order.
    const bookTitle = `E2E Hero CTA ${Date.now()}`;
    const { bookId } = await createBookAndCopy(request, ownerToken, {
      title: bookTitle,
      autoApprove: false,
    });
    const copyRes = await request.post(`${BACKEND_URL}/copies`, {
      headers: { Authorization: `Bearer ${ownerToken}` },
      data: { book_id: bookId, condition: "good", auto_approve: true },
    });
    expect(copyRes.ok()).toBeTruthy();

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);

    // Hero CTA reflects the best copy — auto-approve — even though the
    // request-only copy was created first.
    await expect(
      page.getByRole("button", { name: "Borrow instantly" }),
    ).toBeVisible();
    // A "Best pick" hint is anchored to the same copy so the user
    // can see *which* copy they're about to act on.
    await expect(page.getByText(/Best pick/)).toBeVisible();

    // The request-only copy still gets its own CTA below, so a user
    // who explicitly wants a specific copy isn't forced through the
    // hero shortcut. Its label differentiates it from the auto-approve
    // one — copy-level ("Request to Borrow") vs hero ("Borrow
    // instantly") — so the two buttons don't collide in strict-mode
    // role queries.
    await expect(
      page.getByRole("button", { name: "Request to Borrow" }),
    ).toBeVisible();

    // Breadcrumb back-nav uses a real <a href="/catalog"> rather than
    // router.back(), so a fresh visitor from a QR scan / shared link
    // (empty history stack) still gets home.
    await page
      .getByRole("navigation", { name: "Breadcrumb" })
      .getByRole("link", { name: /back to catalog/i })
      .click();
    await expect(page).toHaveURL(/\/catalog(\?|$)/);
  });

  test("a borrow request left at its default proposes a return date 30 days out", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Default Return Date ${Date.now()}`,
    });

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    // The return-date field is always shown now and prefilled to +30 days —
    // leave it untouched to prove the default actually reaches the backend.
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { status: string; expected_return_date: string }[] =
      await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected a pending request to exist");

    const expectedDate = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    expect(pending.expected_return_date.slice(0, 10)).toBe(expectedDate);
  });

  test("the copy-setup form no longer offers a return-date-required toggle", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/share");
    await page.getByText(/Can't find your book\? Enter manually/i).click();
    await expect(
      page.getByRole("heading", { name: "Enter book manually" }),
    ).toBeVisible();

    // The other two CopySettings toggles are still there, confirming this
    // assertion is looking at the right panel rather than an empty page.
    await expect(page.getByText("Auto-approve requests")).toBeVisible();
    await expect(page.getByText("Stay anonymous")).toBeVisible();
    await expect(page.getByText("Require return date")).toHaveCount(0);
  });

  test("the borrower can change the return date after acceptance, and the owner is notified", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const bookTitle = `E2E Borrower Amends Date ${Date.now()}`;
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: bookTitle,
    });

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { id: number; status: string }[] = await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected a pending request to exist");

    const acceptRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "accepted" },
      },
    );
    expect(acceptRes.ok()).toBeTruthy();

    const newDate = new Date(Date.now() + 45 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);

    await page.goto("/loans");
    const row = page.getByRole("row", { name: bookTitle });
    await row.getByRole("button", { name: "Edit return date" }).click();
    await page.locator("#edit-return-date").fill(newDate);
    await page.getByRole("button", { name: "Save" }).click();
    await expect(
      page.getByText(/Return date updated — .+ has been notified/),
    ).toBeVisible();
    await expect(row.getByText(/Amended by .+/)).toBeVisible();

    const contextOwner = await browser.newContext();
    const ownerPage = await contextOwner.newPage();
    await login(ownerPage, ownerEmail, E2E_TEST_USER_PASSWORD);
    await ownerPage.goto("/notifications");
    await expect(ownerPage.getByText("Return date changed")).toBeVisible();

    await ownerPage.goto(`/my-books/${copyId}/requests`);
    const ownerRow = ownerPage.getByRole("row", { name: /Borrower/ });
    await expect(ownerRow.getByText(/Amended by .+/)).toBeVisible();

    await contextOwner.close();
  });

  test("the owner can change the return date after acceptance, and the borrower is notified", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const bookTitle = `E2E Owner Amends Date ${Date.now()}`;
    const { bookId, copyId } = await createBookAndCopy(request, ownerToken, {
      title: bookTitle,
    });

    await login(page, borrowerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto(`/catalog/${bookId}`);
    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await page.getByRole("button", { name: "Send Request" }).click();
    await expect(page.getByText("Borrow request sent!")).toBeVisible();

    const listRes = await request.get(
      `${BACKEND_URL}/loan-requests?copy_id=${copyId}`,
      { headers: { Authorization: `Bearer ${ownerToken}` } },
    );
    const requests: { id: number; status: string }[] = await listRes.json();
    const pending = requests.find((r) => r.status === "pending");
    if (!pending) throw new Error("expected a pending request to exist");

    const acceptRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "accepted" },
      },
    );
    expect(acceptRes.ok()).toBeTruthy();

    const newDate = new Date(Date.now() + 60 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);

    const contextOwner = await browser.newContext();
    const ownerPage = await contextOwner.newPage();
    await login(ownerPage, ownerEmail, E2E_TEST_USER_PASSWORD);
    await ownerPage.goto(`/my-books/${copyId}/requests`);

    const ownerRow = ownerPage.getByRole("row", { name: /Borrower/ });
    await ownerRow.getByRole("button", { name: "Edit return date" }).click();
    await ownerPage.locator("#edit-return-date").fill(newDate);
    await ownerPage.getByRole("button", { name: "Save" }).click();
    await expect(
      ownerPage.getByText(/Return date updated — .+ has been notified/),
    ).toBeVisible();
    await expect(ownerRow.getByText(/Amended by .+/)).toBeVisible();

    await page.goto("/notifications");
    await expect(page.getByText("Return date changed")).toBeVisible();

    await page.goto("/loans");
    const row = page.getByRole("row", { name: bookTitle });
    await expect(row.getByText(/Amended by .+/)).toBeVisible();

    await contextOwner.close();
  });

  test("changing the return date is rejected once a loan reaches a terminal status", async ({
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const { copyId } = await createBookAndCopy(request, ownerToken, {
      title: `E2E Terminal Date Guard ${Date.now()}`,
    });

    const borrowerLogin = await apiLogin(
      request,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
    );
    const tomorrow = new Date(Date.now() + 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    const createRes = await request.post(`${BACKEND_URL}/loan-requests`, {
      headers: { Authorization: `Bearer ${borrowerLogin.token}` },
      data: { copy_id: copyId, expected_return_date: tomorrow },
    });
    expect(
      createRes.ok(),
      `create request failed: ${await createRes.text()}`,
    ).toBeTruthy();
    const created = await createRes.json();

    const cancelRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${created.id}`,
      {
        headers: { Authorization: `Bearer ${borrowerLogin.token}` },
        data: { status: "cancelled" },
      },
    );
    expect(cancelRes.ok()).toBeTruthy();

    const newDate = new Date(Date.now() + 60 * 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    const dateRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${created.id}/expected-return-date`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { expected_return_date: newDate },
      },
    );
    expect(dateRes.status()).toBe(400);
  });

  test("the owner's Loans → Lending tab splits a still-accepted loan into Current and a returned one into History", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "not viewport-dependent — see the header comment",
    );
    const currentTitle = `E2E Lending Current ${Date.now()}`;
    const historyTitle = `E2E Lending History ${Date.now()}`;
    const { copyId: currentCopyId } = await createBookAndCopy(
      request,
      ownerToken,
      { title: currentTitle },
    );
    const { copyId: historyCopyId } = await createBookAndCopy(
      request,
      ownerToken,
      { title: historyTitle },
    );

    const borrowerLogin = await apiLogin(
      request,
      borrowerEmail,
      E2E_TEST_USER_PASSWORD,
    );
    const tomorrow = new Date(Date.now() + 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);

    async function requestAndAccept(copyId: number): Promise<number> {
      const createRes = await request.post(`${BACKEND_URL}/loan-requests`, {
        headers: { Authorization: `Bearer ${borrowerLogin.token}` },
        data: { copy_id: copyId, expected_return_date: tomorrow },
      });
      expect(
        createRes.ok(),
        `create request failed: ${await createRes.text()}`,
      ).toBeTruthy();
      const created = await createRes.json();

      const acceptRes = await request.patch(
        `${BACKEND_URL}/loan-requests/${created.id}`,
        {
          headers: { Authorization: `Bearer ${ownerToken}` },
          data: { status: "accepted" },
        },
      );
      expect(acceptRes.ok()).toBeTruthy();
      return created.id as number;
    }

    await requestAndAccept(currentCopyId);
    const historyRequestId = await requestAndAccept(historyCopyId);

    const returnRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${historyRequestId}`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { status: "returned" },
      },
    );
    expect(returnRes.ok()).toBeTruthy();

    await login(page, ownerEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/loans");
    await page.getByRole("tab", { name: "Lending" }).click();

    // Current (default) shows the still-accepted loan, not the returned one.
    await expect(page.getByRole("row", { name: currentTitle })).toBeVisible();
    await expect(page.getByRole("row", { name: historyTitle })).toHaveCount(0);

    // History shows the returned loan, not the still-accepted one. The
    // Current/History filter is a SegmentedControl (plain buttons), not a
    // Tabs primitive — only the outer Borrowing/Lending split uses role="tab".
    await page.getByRole("button", { name: "History", exact: true }).click();
    await expect(page.getByRole("row", { name: historyTitle })).toBeVisible();
    await expect(page.getByRole("row", { name: currentTitle })).toHaveCount(0);

    // No action buttons anywhere on this read-only tab.
    await expect(page.getByRole("button", { name: /accept/i })).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Mark as Returned" }),
    ).toHaveCount(0);
  });
});
