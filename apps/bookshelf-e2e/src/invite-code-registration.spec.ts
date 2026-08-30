import {
  test,
  expect,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import {
  E2E_ADMIN_EMAIL,
  E2E_ADMIN_PASSWORD,
  E2E_TEST_USER_PASSWORD,
} from "./test-users";
import { login, registerTestUser } from "./auth-helpers";

// Covers apps/bookshelf/docs/invite-code-spec.md: a verified member's
// permanent invite link, the registration-page banner it produces, and the
// profile/admin management UI around it. The server-side approval-gate
// bypass itself is covered by TestRegisterViaEmailOTP_WithInviteCode in
// bookshelf-backend — deliberately not re-proven here by flipping
// require_registration_approval/allow_registration, since those are
// backend-global settings and this suite runs many spec files concurrently
// against one shared backend (see the first test's comment for why).

const BACKEND_URL = "http://localhost:8000";

async function getToken(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<string> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email, password },
  });
  expect(res.ok(), `login failed: ${await res.text()}`).toBeTruthy();
  return (await res.json()).token as string;
}

async function validateInviteCode(
  request: APIRequestContext,
  code: string,
): Promise<{ valid: boolean; inviter_name: string }> {
  const res = await request.get(`${BACKEND_URL}/auth/invite/${code}`);
  expect(res.ok()).toBeTruthy();
  return res.json();
}

// Reads the invite code out of the tail of a full /register?invite=<code> URL.
function codeFromInviteUrl(url: string): string {
  const code = new URL(url).searchParams.get("invite");
  expect(code, `expected an invite param in ${url}`).toBeTruthy();
  return code ?? "";
}

// Fills and submits the registration details form.
async function fillDetails(
  page: Page,
  name: string,
  email: string,
  password: string,
) {
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);
  await page.getByLabel("Confirm password").fill(password);
  await page.getByRole("button", { name: "Continue" }).click();
}

// Reads the dev-mode debug OTP code shown on the verify-email step (no SMTP
// is configured in this run — see playwright.config.ts) and submits it,
// finishing registration the same way a real user typing the emailed code
// would.
async function completeEmailVerification(page: Page) {
  await expect(page.getByText("Verify your email")).toBeVisible();
  const debugText = await page
    .locator("p", { hasText: "Dev mode" })
    .locator("strong")
    .textContent();
  const code = debugText?.trim() ?? "";
  expect(code, "dev debug code should be present").toBeTruthy();
  await page.getByLabel("Verification code").fill(code);
  await page.getByRole("button", { name: "Verify and create account" }).click();
}

test.describe("invite-code registration", () => {
  test("a member's invite link shows the banner, attributes the new account, and stays usable for the next invitee", async ({
    page,
    request,
    browser,
  }, testInfo) => {
    // Deliberately does NOT flip require_registration_approval/
    // allow_registration here: those are backend-global admin settings, and
    // this suite runs many spec files concurrently against one shared
    // backend — a spec in another file registering or logging in a user
    // while this test held require_registration_approval=true would get an
    // unexpected pending-approval account. The bypass itself (a valid invite
    // still creates an active, unapproved account even with both gates
    // closed) is covered without any shared-state risk by
    // TestRegisterViaEmailOTP_WithInviteCode in
    // bookshelf-backend/internal/handlers/auth_test.go. What this spec
    // proves instead — the part only an end-to-end run can — is that the
    // frontend actually reads ?invite=, shows the right banner, and threads
    // invite_code through to send-email-otp: invited_by_id being set on the
    // resulting account is proof the code made the round trip.
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;
    const inviterEmail = `invite-inviter-${suffix}@example.com`;
    const inviterName = `Inviter ${suffix}`;
    await registerTestUser(
      request,
      inviterEmail,
      E2E_TEST_USER_PASSWORD,
      inviterName,
    );

    // Inviter fetches their link from their own profile page.
    await login(page, inviterEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/profile");
    await page.getByRole("tab", { name: "Invite" }).click();
    const linkInput = page.locator("input[readonly]");
    await expect(linkInput).toBeVisible({ timeout: 10_000 });
    const inviteUrl = await linkInput.inputValue();
    expect(inviteUrl).toContain("/register?invite=");
    const code = codeFromInviteUrl(inviteUrl);

    // First invitee opens the link in a separate, unauthenticated context.
    const inviteeContext = await browser.newContext();
    const inviteePage = await inviteeContext.newPage();
    const inviteeEmail = `invite-invitee1-${suffix}@example.com`;
    try {
      await inviteePage.goto(inviteUrl);
      await expect(
        inviteePage.getByText(new RegExp(`Invited by ${inviterName}`)),
      ).toBeVisible();

      await fillDetails(
        inviteePage,
        "Invitee One",
        inviteeEmail,
        E2E_TEST_USER_PASSWORD,
      );
      await completeEmailVerification(inviteePage);

      await expect(inviteePage).toHaveURL("/catalog");
      expect(
        await inviteePage.evaluate(() =>
          localStorage.getItem("bookshelf_token"),
        ),
      ).toBeTruthy();
    } finally {
      await inviteeContext.close();
    }

    // The link is multi-use and still live for the next invitee.
    const { valid: stillValid, inviter_name: stillInviterName } =
      await validateInviteCode(request, code);
    expect(stillValid).toBe(true);
    expect(stillInviterName).toBe(inviterName);

    // The account is attributed to the inviter — proof invite_code actually
    // reached the backend, not just that a normal signup happened to work.
    const adminToken = await getToken(
      request,
      E2E_ADMIN_EMAIL,
      E2E_ADMIN_PASSWORD,
    );
    const usersRes = await request.get(
      `${BACKEND_URL}/admin/users?page_size=100`,
      { headers: { Authorization: `Bearer ${adminToken}` } },
    );
    const { items } = (await usersRes.json()) as {
      items: Array<{
        email: string;
        pending_approval: boolean;
        invited_by?: { name: string };
      }>;
    };
    const invitee = items.find((u) => u.email === inviteeEmail);
    expect(invitee, "invitee should exist").toBeTruthy();
    expect(invitee?.pending_approval).toBe(false);
    expect(invitee?.invited_by?.name).toBe(inviterName);
  });

  test("an invalid or unknown invite code shows a fallback notice instead of blocking signup", async ({
    page,
  }) => {
    await page.goto("/register?invite=not-a-real-code");
    await expect(
      page.getByText("This invite link is no longer valid"),
    ).toBeVisible();
    // The form remains fully usable — this is a notice, not a hard error.
    await expect(page.getByLabel("Name")).toBeEnabled();
  });

  test("regenerating a link invalidates the old code, and an admin can revoke the new one", async ({
    page,
    request,
  }, testInfo) => {
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;
    const inviterEmail = `invite-regen-${suffix}@example.com`;
    const inviterName = `Regen Inviter ${suffix}`;
    await registerTestUser(
      request,
      inviterEmail,
      E2E_TEST_USER_PASSWORD,
      inviterName,
    );

    await login(page, inviterEmail, E2E_TEST_USER_PASSWORD);
    await page.goto("/profile");
    await page.getByRole("tab", { name: "Invite" }).click();
    const linkInput = page.locator("input[readonly]");
    await expect(linkInput).toBeVisible({ timeout: 10_000 });
    const oldCode = codeFromInviteUrl(await linkInput.inputValue());

    await page.getByRole("button", { name: "Regenerate link" }).click();
    await expect(async () => {
      const newValue = await linkInput.inputValue();
      expect(codeFromInviteUrl(newValue)).not.toBe(oldCode);
    }).toPass({ timeout: 5_000 });
    const newCode = codeFromInviteUrl(await linkInput.inputValue());

    expect((await validateInviteCode(request, oldCode)).valid).toBe(false);
    expect((await validateInviteCode(request, newCode)).valid).toBe(true);

    // Admin revokes the new code from its own Invites page (under
    // Users & Access — split out of the Users page so it's not buried
    // behind the whole member table).
    await login(page, E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD);
    await page.goto("/admin/invites");
    const row = page.getByRole("row", {
      name: new RegExp(inviterName),
    });
    await expect(row).toBeVisible({ timeout: 10_000 });
    page.once("dialog", (dialog) => dialog.accept());
    await row.getByRole("button", { name: "Revoke" }).click();
    await expect(row).toHaveCount(0);

    expect((await validateInviteCode(request, newCode)).valid).toBe(false);
  });
});
