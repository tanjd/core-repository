import { defineConfig, devices } from "@playwright/test";
import { nxE2EPreset } from "@nx/playwright/preset";
import { workspaceRoot } from "@nx/devkit";

// For CI, you may want to set BASE_URL to the deployed application.
const baseURL = process.env["BASE_URL"] || "http://localhost:3000";

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  ...nxE2EPreset(__filename, { testDir: "./src" }),
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    baseURL,
    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",
  },
  /* Dev server, not a production build — the inferred `start` target runs
   * `next start` without building first, which fails when no prior build
   * exists (e.g. a clean CI checkout). Invokes `next dev` directly rather
   * than `nx run food-maps:serve`: the latter runs through Nx's
   * continuous-task executor, which forks its own `run-executor.js` helper
   * to launch `next dev` — when Playwright tree-kills the process it spawned
   * on teardown, that helper (and the `next dev`/next-server processes under
   * it) can be left orphaned instead of dying with it. Calling `next dev`
   * directly makes it the process Playwright owns, so teardown reliably
   * kills the whole tree. */
  webServer: {
    command: "pnpm exec next dev apps/food-maps --port 3000 --webpack",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
    cwd: workspaceRoot,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },

    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },

    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },

    // Uncomment for mobile browsers support
    /* {
      name: 'Mobile Chrome',
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'Mobile Safari',
      use: { ...devices['iPhone 12'] },
    }, */

    // Uncomment for branded browsers
    /* {
      name: 'Microsoft Edge',
      use: { ...devices['Desktop Edge'], channel: 'msedge' },
    },
    {
      name: 'Google Chrome',
      use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    } */
  ],
});
