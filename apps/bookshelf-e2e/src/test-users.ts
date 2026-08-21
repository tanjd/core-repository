// Shared across auth.setup.ts and any spec that needs to log in — kept in a
// plain module (not *.setup.ts/*.spec.ts) since Playwright disallows
// importing one test file from another.
export const E2E_ADMIN_EMAIL = "e2e-admin@bookshelf.local";
export const E2E_ADMIN_PASSWORD = "E2eBookshelfRunner42";
export const E2E_ADMIN_NAME = "E2E Admin";

// Standard password for any one-off test user a spec registers via
// registerTestUser() (distinct from E2E_ADMIN_PASSWORD above, which is only
// for the seeded admin account). Meets auth.go's validatePasswordComplexity
// (12+ chars, mixed case + digit, not derived from the user's name/email) —
// reuse this instead of inventing a fresh literal per spec, so anyone poking
// at a leftover test user (e.g. logging in manually against a
// reuseExistingServer backend) doesn't have to go dig up which spec created
// it. A spec that needs a second, distinct password (e.g. to prove a
// password-change/reset actually took effect) should still use its own
// literal for that second one.
export const E2E_TEST_USER_PASSWORD = "E2eTestUserPassw0rd1";
