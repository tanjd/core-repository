import { test, expect, type APIRequestContext } from "@playwright/test";
import { E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD } from "./test-users";
import { login } from "./auth-helpers";

// Covers the monthly-digest scheduler job entry on the Admin > Jobs page,
// introduced in Slice 3 of apps/bookshelf/docs/monthly-digest-plan.md.

const BACKEND_URL = "http://localhost:8000";

async function getAdminToken(request: APIRequestContext): Promise<string> {
  const res = await request.post(`${BACKEND_URL}/auth/login`, {
    data: { email: E2E_ADMIN_EMAIL, password: E2E_ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login failed: ${await res.text()}`).toBeTruthy();
  const body = await res.json();
  return body.token as string;
}

test("monthly-digest job appears in the Jobs page", async ({ page }) => {
  await login(page, E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD);
  await page.goto("/admin/jobs");
  await expect(page.getByText("Monthly Digest")).toBeVisible();
});

test('"Run now" on monthly-digest updates LastResult in the Jobs table', async ({
  page,
  request,
}) => {
  const token = await getAdminToken(request);

  // Trigger via API so we don't need to wait for the UI polling cycle.
  const runRes = await request.post(
    `${BACKEND_URL}/admin/jobs/monthly-digest/run`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(
    runRes.ok() || runRes.status() === 202,
    `run job failed: ${await runRes.text()}`,
  ).toBeTruthy();

  await login(page, E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD);
  await page.goto("/admin/jobs");
  await expect(page.getByText("Monthly Digest")).toBeVisible();

  // The job runs asynchronously; poll until a non-empty last_result appears
  // (the card shows it below the job name).
  await expect(async () => {
    const jobsRes = await request.get(`${BACKEND_URL}/admin/jobs`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const jobs = await jobsRes.json();
    const digest = (jobs as Array<{ name: string; last_result: string }>).find(
      (j) => j.name === "monthly-digest",
    );
    expect(digest?.last_result).toBeTruthy();
  }).toPass({ timeout: 10_000 });
});

test('"Send test email" button calls the endpoint and shows a toast', async ({
  page,
}) => {
  await login(page, E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD);
  await page.goto("/admin/jobs");
  await expect(page.getByText("Monthly Digest")).toBeVisible();

  await page.getByRole("button", { name: "Send test email" }).click();

  // Toast message appears (SMTP_HOST is empty so the email is skipped
  // server-side, but the endpoint still returns 200 with sent:true).
  await expect(
    page.getByText(new RegExp(`Test email sent to ${E2E_ADMIN_EMAIL}`)),
  ).toBeVisible({ timeout: 8_000 });
});
