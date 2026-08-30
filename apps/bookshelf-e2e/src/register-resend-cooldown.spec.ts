import { test, expect } from "@playwright/test";

// "Resend code" on the verify-email step used to have no debounce: a user
// who didn't see the email within a few seconds could tap it repeatedly,
// flooding their inbox or silently tripping the backend rate limit. It's now
// disabled for 30 seconds, counting down, immediately on reaching this step.
// Uses page.clock to fast-forward those 30 seconds instead of waiting for
// them in real time.
test("Resend code is disabled with a 30s countdown, then re-enables", async ({
  page,
}, testInfo) => {
  await page.clock.install({ time: Date.now() });

  const suffix = `${testInfo.project.name.replace(/\s+/g, "-")}-${Date.now()}`;
  await page.goto("/register");

  await page.getByLabel("Name").fill("Resend Cooldown Tester");
  await page.getByLabel("Email").fill(`resend-cooldown-${suffix}@example.com`);
  await page
    .getByLabel("Password", { exact: true })
    .fill("ResendCooldownPassw0rd1");
  await page.getByLabel("Confirm password").fill("ResendCooldownPassw0rd1");
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByText("Verify your email")).toBeVisible();

  const resendButton = page.getByRole("button", { name: /Resend/ });
  await expect(resendButton).toBeDisabled();
  await expect(resendButton).toHaveText("Resend in 0:30");

  // Each tick's next setTimeout is only (re-)armed once React commits the
  // state update from the previous one, which needs a real event-loop turn
  // the virtual clock doesn't control — so a single runFor(30_000) doesn't
  // reliably chain through all 30 re-armed timeouts. Advancing one second at
  // a time, awaiting each, gives that commit+effect cycle room to run.
  for (let i = 0; i < 30; i++) {
    await page.clock.runFor(1000);
  }

  await expect(resendButton).toBeEnabled();
  await expect(resendButton).toHaveText("Resend code");
});
