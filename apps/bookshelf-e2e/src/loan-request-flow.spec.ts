import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the loan-request UX fixes described in
// apps/bookshelf/docs/loan-request-ux-fixes-spec.md, end-to-end against the
// real backend — this is the first coverage this suite has for the
// borrow/loan/return flow (apps/bookshelf-e2e/CLAUDE.md's "Coverage" section
// previously called this out as untested).
//
// All setup/verification calls hit the backend directly on :8000 (same
// pattern as auth-helpers.ts's registerTestUser), bypassing the frontend's
// /api proxy — these routes carry no "/api" prefix on the backend itself.
//
// Only two accounts (owner, borrower) are registered, once in beforeAll,
// and reused across all four tests below — including for the "a second
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
    returnDateRequired?: boolean;
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
      return_date_required: opts.returnDateRequired ?? false,
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

    await expect(page.getByText("Instant approval")).toBeVisible();

    await page.getByRole("button", { name: "Request to Borrow" }).click();
    await expect(
      page.getByText(/auto-approves.*owner's contact info right away/i),
    ).toBeVisible();
    await page.getByRole("button", { name: "Send Request" }).click();

    await expect(
      page.getByText(
        "Request approved — check My Requests for the owner's contact info",
      ),
    ).toBeVisible();

    await page.goto("/my-requests");
    await expect(page.getByText("accepted", { exact: true })).toBeVisible();
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
    // covers the later "view My Requests" step below.
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

    const yesterday = new Date(Date.now() - 24 * 60 * 60 * 1000)
      .toISOString()
      .slice(0, 10);
    const dateRes = await request.patch(
      `${BACKEND_URL}/loan-requests/${pending.id}/expected-return-date`,
      {
        headers: { Authorization: `Bearer ${ownerToken}` },
        data: { expected_return_date: yesterday },
      },
    );
    expect(dateRes.ok()).toBeTruthy();

    await page.goto("/my-requests");
    await expect(page.getByText("Overdue", { exact: true })).toBeVisible();

    const contextOwner = await browser.newContext();
    const ownerPage = await contextOwner.newPage();
    await login(ownerPage, ownerEmail, E2E_TEST_USER_PASSWORD);
    await ownerPage.goto("/my-books");
    await expect(ownerPage.getByText(/overdue since/i)).toBeVisible();

    await ownerPage.goto(`/my-books/${copyId}/requests`);
    await expect(ownerPage.getByText("Overdue", { exact: true })).toBeVisible();

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
      returnDateRequired: true,
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
    await expect(
      ownerPage.getByText("accepted", { exact: true }),
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

    await page.goto("/my-requests");
    // Scoped to this test's own row: borrowerEmail is shared across every
    // test in this file (see the header comment), so by this 8th test
    // /my-requests already has other "accepted" rows from earlier tests —
    // a bare page-wide getByText("accepted") is a strict-mode violation.
    const row = page.getByRole("row", { name: bookTitle });
    await expect(row.getByText("accepted", { exact: true })).toBeVisible();
    await row.getByText(bookTitle).click();
    await expect(page.getByText("@e2e_owner")).toBeVisible();
    await expect(
      page.getByText("meet at the front desk on Saturdays"),
    ).toBeVisible();
  });
});
