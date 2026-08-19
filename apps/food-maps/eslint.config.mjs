import { defineConfig, globalIgnores } from "eslint/config";
import nextCoreWebVitals from "eslint-config-next/core-web-vitals";

const config = defineConfig([
  ...nextCoreWebVitals,
  {
    files: ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx"],
    rules: {},
  },
  // Override default ignores of eslint-config-next. Nx's @nx/eslint:lint
  // executor runs eslint with cwd set to the workspace root while passing
  // this file as an explicit --config path, so un-prefixed patterns like
  // ".next/**" only match a .next dir directly under the workspace root —
  // not apps/food-maps/.next. Leading "**/" makes them cwd-independent.
  globalIgnores([
    // Default ignores of eslint-config-next:
    "**/.next/**",
    "**/out/**",
    "**/build/**",
    "next-env.d.ts",
  ]),
]);

export default config;
