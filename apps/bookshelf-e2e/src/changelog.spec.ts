import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { login } from "./auth-helpers";
import { E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD } from "./test-users";

const appVersion = (
  JSON.parse(
    readFileSync(join(__dirname, "../../bookshelf/package.json"), "utf8"),
  ) as { version: string }
).version;

test("footer version links to the changelog page", async ({ page }) => {
  await page.goto("/login");

  await page.getByRole("link", { name: `v${appVersion}` }).click();
  await expect(page).toHaveURL("/changelog");
  await expect(page.getByRole("heading", { name: "Changelog" })).toBeVisible();
  await expect(
    page.getByText(
      "What's new in Bookshelf — release notes for members and admins.",
    ),
  ).toBeVisible();
});

test("changelog page renders release notes from the latest entry", async ({
  page,
}) => {
  await page.goto("/changelog");

  await expect(page.getByText("Current release")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: `v${appVersion}` }).first(),
  ).toBeVisible();
  await expect(page.getByText("Features").first()).toBeVisible();
});

test("upgrade notice appears in the notification panel and can be dismissed", async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem("bookshelf_last_seen_app_version", "0.0.1");
  });

  await login(page, E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD);

  const bell = page.getByRole("button", { name: /^Notifications/ });
  await expect(bell).toHaveAttribute(
    "aria-label",
    /Notifications \(1 unread\)/,
  );
  await bell.click();

  await expect(page.getByText(`What's new in v${appVersion}`)).toBeVisible();
  await expect(
    page.getByRole("link", { name: "View release notes" }),
  ).toBeVisible();
  await page.getByLabel("Dismiss update notice").click();
  await expect(bell).toHaveAttribute("aria-label", "Notifications");
});
