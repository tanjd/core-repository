import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  formatSchemaMigration,
  parseChangelogMarkdown,
  semverGt,
} from "./changelog";

describe("semverGt", () => {
  it("returns true when major version is higher", () => {
    expect(semverGt("1.0.0", "0.9.9")).toBe(true);
  });

  it("returns true when minor or patch is higher", () => {
    expect(semverGt("0.22.0", "0.21.0")).toBe(true);
    expect(semverGt("0.22.1", "0.22.0")).toBe(true);
  });

  it("returns false for equal or lower versions", () => {
    expect(semverGt("0.22.0", "0.22.0")).toBe(false);
    expect(semverGt("0.21.0", "0.22.0")).toBe(false);
  });
});

describe("parseChangelogMarkdown", () => {
  it("parses # and ## semver headers with optional dates", () => {
    const entries = parseChangelogMarkdown(`
## 0.22.0 (2026-08-25)

### Features

- first item

## 0.21.0 (2026-08-24)

- second release

# 0.20.0

- third release
`);

    expect(entries).toHaveLength(3);
    expect(entries[0]).toMatchObject({
      version: "0.22.0",
      date: "2026-08-25",
    });
    expect(entries[0].body).toContain("first item");
    expect(entries[2].version).toBe("0.20.0");
    expect(entries[2].date).toBeNull();
  });

  it("extracts entries from the real CHANGELOG.md", () => {
    const content = readFileSync(join(__dirname, "../../CHANGELOG.md"), "utf8");
    const entries = parseChangelogMarkdown(content);
    expect(entries.length).toBeGreaterThan(0);
    expect(entries[0].version).toMatch(/^\d+\.\d+\.\d+$/);
    expect(entries[0].body.length).toBeGreaterThan(0);
  });
});

describe("formatSchemaMigration", () => {
  it("zero-pads migration numbers to six digits", () => {
    expect(formatSchemaMigration(13)).toBe("000013");
  });
});
