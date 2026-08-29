import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the one-click unsubscribe flow introduced in Slice 2 of
// apps/bookshelf/docs/monthly-digest-plan.md.
//
// Uses the dev-only GET /unsubscribe/digest/debug-token endpoint to mint a
// token without needing real SMTP — same pattern as auth debug_code fields.

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

async function mintUnsubscribeToken(
  request: APIRequestContext,
  userId: number,
): Promise<string> {
  const res = await request.get(
    `${BACKEND_URL}/unsubscribe/digest/debug-token?user_id=${userId}`,
  );
  expect(res.ok(), `token mint failed: ${await res.text()}`).toBeTruthy();
  const body = await res.json();
  return body.token as string;
}

async function getMe(
  request: APIRequestContext,
  token: string,
): Promise<{ monthly_digest_enabled: boolean }> {
  const res = await request.get(`${BACKEND_URL}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

test.describe("Monthly digest unsubscribe", () => {
  // Tests below share one user (registered once in beforeAll) and build on
  // each other's mutations (e.g. the idempotent-click test relies on the
  // first test having already unsubscribed) — force them onto one worker in
  // declaration order so fullyParallel (the Nx Playwright preset default)
  // can't split beforeAll across workers and hand different tests different
  // users.
  test.describe.configure({ mode: "serial" });

  let userEmail: string;
  let userId: number;

  test.beforeAll(async ({ request }) => {
    userEmail = `digest-unsub-e2e-${Date.now()}@example.com`;
    await registerTestUser(request, userEmail, E2E_TEST_USER_PASSWORD);
    const { userId: id } = await apiLogin(
      request,
      userEmail,
      E2E_TEST_USER_PASSWORD,
    );
    userId = id;
  });

  test("unsubscribe page shows confirmation and flips the flag", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );

    const unsubToken = await mintUnsubscribeToken(request, userId);
    await page.goto(`/unsubscribe?token=${unsubToken}`);

    await expect(page.getByText(/you're unsubscribed/i)).toBeVisible();
    await expect(page.getByText(userEmail)).toBeVisible();
    await expect(
      page.getByText(/re-enable it any time from your profile settings/i),
    ).toBeVisible();

    const { token: apiToken } = await apiLogin(
      request,
      userEmail,
      E2E_TEST_USER_PASSWORD,
    );
    const me = await getMe(request, apiToken);
    expect(me.monthly_digest_enabled).toBe(false);
  });

  test("unsubscribe page is idempotent — clicking again shows confirmation", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );

    // User is already unsubscribed from the previous test; mint a fresh token
    const unsubToken = await mintUnsubscribeToken(request, userId);
    await page.goto(`/unsubscribe?token=${unsubToken}`);

    await expect(page.getByText(/you're unsubscribed/i)).toBeVisible();

    const { token: apiToken } = await apiLogin(
      request,
      userEmail,
      E2E_TEST_USER_PASSWORD,
    );
    const me = await getMe(request, apiToken);
    expect(me.monthly_digest_enabled).toBe(false);
  });

  test("invalid token shows error state", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );

    await page.goto("/unsubscribe?token=not-a-valid-token");

    await expect(page.getByText(/link not valid/i)).toBeVisible();
  });

  test("missing token shows error state", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );

    await page.goto("/unsubscribe");

    await expect(page.getByText(/link not valid/i)).toBeVisible();
  });

  test("profile toggle reflects the unsubscribed state after login", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );

    await login(page, userEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/profile");

    await expect(
      page.getByRole("switch", { name: /monthly digest/i }),
    ).not.toBeChecked();
  });
});
