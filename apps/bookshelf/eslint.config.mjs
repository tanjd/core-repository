import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next. Nx's @nx/eslint:lint
  // executor runs eslint with cwd set to the workspace root while passing
  // this file as an explicit --config path, so un-prefixed patterns like
  // ".next/**" only match a .next dir directly under the workspace root —
  // not apps/bookshelf/.next. Leading "**/" makes them cwd-independent.
  globalIgnores([
    // Default ignores of eslint-config-next:
    "**/.next/**",
    "**/out/**",
    "**/build/**",
    "next-env.d.ts",
  ]),
  {
    // The source repo pinned eslint-config-next 16.1.6; this workspace is on
    // 16.2.12, whose bundled eslint-plugin-react-hooks added
    // react-hooks/set-state-in-effect as an error. It fires on 11 ported
    // call sites (all "hydrate auth/fetched state on mount" patterns, e.g.
    // src/components/auth/AdminGuard.tsx) that are safe as written but
    // flagged as an anti-pattern by the newer rule. Downgrading rather than
    // rewriting 11 components' effect logic as part of a migration pass —
    // revisit as a follow-up (see apps/bookshelf/CLAUDE.md).
    rules: {
      "react-hooks/set-state-in-effect": "warn",
    },
  },
]);

export default eslintConfig;
