import { defineConfig, globalIgnores } from "eslint/config";
import tsParser from "@typescript-eslint/parser";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    settings: { react: { version: "19.1.1" } },
  },
  {
    files: ["**/*.{js,mjs,cjs,jsx}"],
    languageOptions: {
      parser: tsParser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
  },
  // Override default ignores of eslint-config-next. Nx's @nx/eslint:lint
  // executor runs eslint with cwd set to the workspace root while passing
  // this file as an explicit --config path, so un-prefixed patterns like
  // ".next/**" only match a .next dir directly under the workspace root —
  // not apps/ledger-lens/.next. Leading "**/" makes them cwd-independent.
  globalIgnores([
    // Default ignores of eslint-config-next:
    "**/.next/**",
    "**/out/**",
    "**/build/**",
    "next-env.d.ts",
  ]),
]);

export default eslintConfig;
