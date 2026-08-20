// Shared across auth.setup.ts and any spec that needs to log in — kept in a
// plain module (not *.setup.ts/*.spec.ts) since Playwright disallows
// importing one test file from another.
export const E2E_ADMIN_EMAIL = "e2e-admin@bookshelf.local";
export const E2E_ADMIN_PASSWORD = "E2eBookshelfRunner42";
export const E2E_ADMIN_NAME = "E2E Admin";
