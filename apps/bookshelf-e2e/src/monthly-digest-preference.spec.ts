import { test, expect, type APIRequestContext } from "@playwright/test";
import { login, registerTestUser } from "./auth-helpers";
import { E2E_TEST_USER_PASSWORD } from "./test-users";

// Covers the monthly digest preference toggle introduced in Slice 1 of
// apps/bookshelf/docs/monthly-digest-plan.md.
//
// Each test registers and drives its own user rather than sharing one across
// tests: this suite's Nx preset runs `fullyParallel: true`, so tests in the
// same describe block aren't guaranteed to execute in declaration order (or
// even sequentially), and a shared user would race across them.

const BACKEND_URL = "http://localhost:8000";
const MOBILE_PROJECT = "Mobile Chrome";

function uniqueEmail(label: string, testInfo: { project: { name: string } }) {
  return `${label}-${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

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

async function setMonthlyDigestEnabled(
  request: APIRequestContext,
  token: string,
  enabled: boolean,
) {
  const res = await request.patch(`${BACKEND_URL}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { monthly_digest_enabled: enabled },
  });
  expect(res.ok()).toBeTruthy();
}

test.describe("Monthly digest preference", () => {
  test("new user defaults to monthly digest enabled", async ({
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );
    const email = uniqueEmail("digest-default", testInfo);
    await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);

    const token = await apiLogin(request, email, E2E_TEST_USER_PASSWORD);
    const me = await getMe(request, token);
    expect(me.monthly_digest_enabled).toBe(true);
  });

  test("profile toggle disables digest and persists across reload", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );
    const email = uniqueEmail("digest-disable", testInfo);
    await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);
    await login(page, email, E2E_TEST_USER_PASSWORD);
    await page.goto("/profile");

    const digestSwitch = page.getByRole("switch", {
      name: /monthly digest/i,
    });
    await expect(digestSwitch).toBeChecked();

    await digestSwitch.click();
    await expect(digestSwitch).not.toBeChecked();

    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByText("Profile updated")).toBeVisible();

    // Reload and confirm the UI reflects the persisted value
    await page.reload();
    await expect(
      page.getByRole("switch", { name: /monthly digest/i }),
    ).not.toBeChecked();

    // Confirm the API also reflects it
    const token = await apiLogin(request, email, E2E_TEST_USER_PASSWORD);
    const me = await getMe(request, token);
    expect(me.monthly_digest_enabled).toBe(false);
  });

  test("profile toggle re-enables digest and persists", async ({
    page,
    request,
  }, testInfo) => {
    test.skip(
      testInfo.project.name === MOBILE_PROJECT,
      "viewport-independent; skip duplicate mobile run",
    );
    const email = uniqueEmail("digest-enable", testInfo);
    await registerTestUser(request, email, E2E_TEST_USER_PASSWORD);

    // Start from a known-disabled state via the API rather than relying on
    // another test's ordering.
    const token = await apiLogin(request, email, E2E_TEST_USER_PASSWORD);
    await setMonthlyDigestEnabled(request, token, false);

    await login(page, email, E2E_TEST_USER_PASSWORD);
    await page.goto("/profile");

    const digestSwitch = page.getByRole("switch", {
      name: /monthly digest/i,
    });
    await expect(digestSwitch).not.toBeChecked();

    await digestSwitch.click();
    await expect(digestSwitch).toBeChecked();

    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByText("Profile updated")).toBeVisible();

    const me = await getMe(request, token);
    expect(me.monthly_digest_enabled).toBe(true);
  });
});
