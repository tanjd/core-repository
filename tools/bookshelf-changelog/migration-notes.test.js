const {
  extractMigrationIds,
  formatMigrationSection,
  injectMigrationSection,
  listNewMigrationFiles,
  shouldAnnotateProject,
} = require("./migration-notes");

describe("shouldAnnotateProject", () => {
  it("returns true for bookshelf projects only", () => {
    expect(shouldAnnotateProject("bookshelf")).toBe(true);
    expect(shouldAnnotateProject("bookshelf-backend")).toBe(true);
    expect(shouldAnnotateProject("index-watch")).toBe(false);
    expect(shouldAnnotateProject(null)).toBe(false);
  });
});

describe("extractMigrationIds", () => {
  it("parses six-digit migration prefixes from file paths", () => {
    expect(
      extractMigrationIds([
        "apps/bookshelf-backend/internal/db/migrations/000014_add_foo.up.sql",
        "apps/bookshelf-backend/internal/db/migrations/000015_add_bar.up.sql",
      ]),
    ).toEqual(["000014", "000015"]);
  });

  it("ignores paths that do not match the migration filename pattern", () => {
    expect(
      extractMigrationIds([
        "apps/bookshelf-backend/internal/db/migrations/README.md",
      ]),
    ).toEqual([]);
  });
});

describe("formatMigrationSection", () => {
  it("returns an empty string when there are no migration files", () => {
    expect(formatMigrationSection([])).toBe("");
  });

  it("builds the database migrations subsection", () => {
    const section = formatMigrationSection([
      "apps/bookshelf-backend/internal/db/migrations/000014_add_foo.up.sql",
    ]);

    expect(section).toContain("### Database migrations");
    expect(section).toContain("**000014**");
    expect(section).toContain("automatic on startup");
  });
});

describe("injectMigrationSection", () => {
  const section = [
    "### Database migrations",
    "",
    "Includes migration **000014** — automatic on startup; no manual SQL required.",
  ].join("\n");

  it("inserts the section before the Thank You block", () => {
    const changelog = [
      "## 0.23.0 (2026-09-01)",
      "",
      "### 🚀 Features",
      "",
      "- item",
      "",
      "### ❤️ Thank You",
      "",
      "- Jeddy Tan",
    ].join("\n");

    const result = injectMigrationSection(changelog, section);
    expect(result.indexOf("### Database migrations")).toBeLessThan(
      result.indexOf("### ❤️ Thank You"),
    );
    expect(result).toContain("- item");
  });

  it("appends the section when there is no Thank You block", () => {
    const changelog = "## 0.23.0\n\n### 🚀 Features\n\n- item";
    expect(injectMigrationSection(changelog, section)).toBe(
      `${changelog}\n\n${section}`,
    );
  });

  it("returns the original changelog when the section is empty", () => {
    const changelog = "## 0.23.0\n\n- item";
    expect(injectMigrationSection(changelog, "")).toBe(changelog);
  });
});

describe("listNewMigrationFiles", () => {
  it("returns added migration files since the latest backend tag", () => {
    const calls = [];
    const files = listNewMigrationFiles((command) => {
      calls.push(command);
      if (command.includes("git describe")) {
        return "bookshelf-backend@0.22.0\n";
      }
      return [
        "apps/bookshelf-backend/internal/db/migrations/000014_add_foo.up.sql",
        "apps/bookshelf-backend/internal/db/migrations/000015_add_bar.up.sql",
      ].join("\n");
    });

    expect(files).toEqual([
      "apps/bookshelf-backend/internal/db/migrations/000014_add_foo.up.sql",
      "apps/bookshelf-backend/internal/db/migrations/000015_add_bar.up.sql",
    ]);
    expect(calls[1]).toContain("bookshelf-backend@0.22.0..HEAD");
    expect(calls[1]).toContain("--diff-filter=A");
  });

  it("returns an empty list when no previous backend tag exists", () => {
    expect(
      listNewMigrationFiles(() => {
        throw new Error("no tag");
      }),
    ).toEqual([]);
  });
});
