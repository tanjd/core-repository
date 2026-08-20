#!/usr/bin/env node
// lint-staged runs plain `eslint` from the repo root, and ESLint's flat
// config only ever loads a single config file — the nearest one to `cwd`,
// not to each linted file — so it always resolved the root eslint.config.mjs
// and never apps/*'s own eslint.config.mjs (which pull in per-project
// plugins like eslint-config-next's eslint-plugin-react-hooks). nx's
// @nx/eslint:lint executor works around this by passing each project's
// config explicitly via --config; this script does the same for
// lint-staged, so a local `git commit` checks staged files against the same
// config CI/nx would use, instead of silently under-linting them (or, worse,
// hard-erroring on an inline eslint-disable comment for a rule that only
// exists in the project-specific config).
import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join, relative } from "node:path";

const files = process.argv.slice(2);
if (files.length === 0) process.exit(0);

// Resolved explicitly rather than left to PATH: lint-staged prepends
// node_modules/.bin when it runs this script, but that isn't true when this
// script is invoked directly (e.g. for local debugging).
const eslintBin = join(process.cwd(), "node_modules", ".bin", "eslint");

function nearestConfig(filePath) {
  let dir = dirname(filePath);
  while (true) {
    const candidate = join(dir, "eslint.config.mjs");
    if (existsSync(candidate)) return candidate;
    const parent = dirname(dir);
    if (parent === dir) return join(process.cwd(), "eslint.config.mjs");
    dir = parent;
  }
}

const groups = new Map();
for (const file of files) {
  const config = nearestConfig(file);
  if (!groups.has(config)) groups.set(config, []);
  groups.get(config).push(file);
}

let failed = false;
for (const [config, groupFiles] of groups) {
  try {
    execFileSync(
      eslintBin,
      ["--config", relative(process.cwd(), config), "--fix", ...groupFiles],
      { stdio: "inherit" },
    );
  } catch {
    failed = true;
  }
}

process.exit(failed ? 1 : 0);
