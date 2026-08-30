import {
  test,
  expect,
  type APIRequestContext,
  type Page,
  type Locator,
} from "@playwright/test";
import {
  E2E_ADMIN_EMAIL,
  E2E_ADMIN_PASSWORD,
  E2E_TEST_USER_PASSWORD,
} from "./test-users";
import { registerTestUser } from "./auth-helpers";

// Covers the admin Users page's row-action overflow menu (replaces 3-4
// always-visible inline buttons per row) and the admin sidebar/nav's
// pending-approval count badge. Admin approval flows had no e2e coverage
// before this — see this suite's CLAUDE.md "Coverage" section.

const BACKEND_URL = "http://localhost:8000";

test.use({ storageState: ".auth/admin.json" });

async function getAdminToken(request: APIRequestContext): Promise<string> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email: E2E_ADMIN_EMAIL, password: E2E_ADMIN_PASSWORD },
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).token as string;
}

async function findUserId(
  request: APIRequestContext,
  token: string,
  email: string,
): Promise<number> {
  const res = await request.get(`${BACKEND_URL}/admin/users?page_size=100`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.ok()).toBeTruthy();
  const { items } = (await res.json()) as {
    items: Array<{ id: number; email: string }>;
  };
  const user = items.find((u) => u.email === email);
  if (!user) throw new Error(`expected to find user ${email}`);
  return user.id;
}

// The Users table paginates at 20/page (users/page.tsx), and this suite runs
// many spec files concurrently against one shared backend — other specs'
// registered users can easily push a freshly-created account past page 1.
// Click through pages (rather than assuming page 1) until the row shows up.
async function findUserRow(page: Page, email: string): Promise<Locator> {
  const row = page.getByRole("row", { name: new RegExp(email) });
  const nextButton = page.getByRole("button", { name: "Next page" });
  // The very first check can otherwise land mid-load, before Pagination (and
  // its "Next page" button) has even rendered — wait the initial fetch out.
  await page
    .getByText("Loading users…")
    .waitFor({ state: "hidden" })
    .catch(() => undefined);
  for (let attempt = 0; attempt < 25; attempt++) {
    if (await row.count()) return row;
    if (!(await nextButton.isEnabled().catch(() => false))) break;
    await nextButton.click();
    await page
      .getByText("Loading users…")
      .waitFor({ state: "hidden" })
      .catch(() => undefined);
  }
  return row;
}

test.describe("admin users row actions", () => {
  test("the overflow menu approves a pending user, and the badge reflects it", async ({
    page,
    request,
  }, testInfo) => {
    // Paging through the Users table under full-suite concurrency (many
    // spec files registering users in parallel) can take a few more round
    // trips than the default timeout allows.
    test.setTimeout(60_000);
    const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;
    const email = `admin-actions-${suffix}@example.com`;
    const name = `Admin Actions ${suffix}`;
    await registerTestUser(request, email, E2E_TEST_USER_PASSWORD, name);

    const adminToken = await getAdminToken(request);
    const userId = await findUserId(request, adminToken, email);

    // Put the freshly-registered account into pending_approval — the same
    // admin PATCH the UI's own "Approve" action drives — so this spec exercises
    // the redesigned overflow menu without depending on the backend-global
    // require_registration_approval setting (which invite-code-registration.spec.ts
    // deliberately avoids flipping, since many spec files share one backend).
    const patchRes = await request.patch(
      `${BACKEND_URL}/admin/users/${userId}`,
      {
        headers: { Authorization: `Bearer ${adminToken}` },
        data: { pending_approval: true },
      },
    );
    expect(patchRes.ok()).toBeTruthy();

    await page.goto("/admin/users");

    // Sidebar (desktop) shows a pending-count badge on the Users nav item.
    const sidebarUsersLink = page.getByRole("link", { name: /^Users/ });
    await expect(sidebarUsersLink).toBeVisible();
    await expect(sidebarUsersLink.getByText(/^\d+$/)).toBeVisible();

    const row = await findUserRow(page, email);
    await expect(row).toBeVisible({ timeout: 10_000 });
    await expect(row.getByText("pending approval")).toBeVisible();

    // Row actions are collapsed behind a single overflow-menu trigger.
    await row.getByRole("button", { name: `Actions for ${name}` }).click();
    await page.getByRole("menuitem", { name: "Approve" }).click();

    await expect(row.getByText("pending approval")).toHaveCount(0, {
      timeout: 10_000,
    });
  });
});
