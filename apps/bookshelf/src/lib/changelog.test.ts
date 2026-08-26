import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  extractMigrationNumber,
  extractTeaserLines,
  filterSectionsForAudience,
  formatChangelogDate,
  formatChangelogDateLabel,
  formatRelativeChangelogDate,
  formatSchemaMigration,
  getLatestChangelogTeaser,
  latestReferencedMigration,
  parseChangelogMarkdown,
  parseChangelogSections,
  semverGt,
  stripMarkdownInline,
  stripPrLinks,
  versionAnchorId,
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

describe("parseChangelogSections", () => {
  it("classifies nx-release section headings", () => {
    const sections = parseChangelogSections(`
### 🚀 Features

- add thing

### Database migrations

Includes migration **000014** — automatic on startup.

### ❤️ Thank You

- Jeddy Tan
`);

    expect(sections.map((section) => section.kind)).toEqual([
      "features",
      "migrations",
      "thank_you",
    ]);
  });
});

describe("stripPrLinks and stripMarkdownInline", () => {
  it("removes PR link suffixes from bullet text", () => {
    const line =
      "- **bookshelf:** require return date ([#80](https://github.com/tanjd/core-repository/pull/80))";
    expect(stripPrLinks(line)).not.toContain("#80");
    expect(stripMarkdownInline(line)).toBe("bookshelf: require return date");
  });
});

describe("extractTeaserLines", () => {
  it("returns the first user-facing bullets and skips thank-you sections", () => {
    const entry = {
      version: "0.24.0",
      date: "2026-08-26",
      body: `
### 🚀 Features

- **bookshelf:** require expected return date ([#80](https://example.com))

### ❤️ Thank You

- Jeddy Tan
`,
    };

    expect(extractTeaserLines(entry, 1)).toEqual([
      "bookshelf: require expected return date",
    ]);
  });
});

describe("migration helpers", () => {
  it("extracts migration numbers and finds the latest reference", () => {
    const entries = parseChangelogMarkdown(`
## 0.24.0 (2026-08-26)

### Database migrations

Includes migration **000014** — automatic on startup.

## 0.23.0 (2026-08-26)

### Database migrations

Includes migration **000013** — automatic on startup.
`);

    expect(extractMigrationNumber(entries[0].body)).toBe(14);
    expect(latestReferencedMigration(entries)).toBe(14);
  });
});

describe("date formatting", () => {
  it("formats absolute and relative changelog dates", () => {
    expect(formatChangelogDate("2026-08-26")).toMatch(/Aug 26, 2026/);

    const today = new Date();
    const iso = today.toISOString().slice(0, 10);
    expect(formatRelativeChangelogDate(iso)).toBe("Today");
    expect(formatChangelogDateLabel(iso, { preferRelative: true })).toBe(
      "Today",
    );
  });
});

describe("audience filtering", () => {
  it("hides thank-you and migration sections for members", () => {
    const sections = parseChangelogSections(`
### 🚀 Features

- feature

### Database migrations

Includes migration **000014**.

### ❤️ Thank You

- Jeddy Tan
`);

    expect(
      filterSectionsForAudience(sections, { showAdminDetails: false }).map(
        (section) => section.kind,
      ),
    ).toEqual(["features"]);
  });
});

describe("getLatestChangelogTeaser", () => {
  it("returns the first teaser from the newest entry", () => {
    const teaser = getLatestChangelogTeaser([
      {
        version: "0.24.0",
        date: null,
        body: "### 🚀 Features\n\n- **bookshelf:** teaser line",
      },
    ]);

    expect(teaser).toBe("bookshelf: teaser line");
  });
});

describe("formatSchemaMigration", () => {
  it("zero-pads migration numbers to six digits", () => {
    expect(formatSchemaMigration(13)).toBe("000013");
  });
});

describe("versionAnchorId", () => {
  it("builds stable hash targets for release sections", () => {
    expect(versionAnchorId("0.24.0")).toBe("v0.24.0");
  });
});
