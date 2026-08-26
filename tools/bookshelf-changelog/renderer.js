const { execSync } = require("node:child_process");
const DefaultChangelogRenderer =
  require("nx/release/changelog-renderer").default;
const {
  formatMigrationSection,
  injectMigrationSection,
  listNewMigrationFiles,
  shouldAnnotateProject,
} = require("./migration-notes");

/** @param {string} command */
function runGit(command) {
  return execSync(command, { encoding: "utf8" });
}

module.exports = class BookshelfChangelogRenderer extends (
  DefaultChangelogRenderer
) {
  async render() {
    const base = await super.render();
    if (!base || !shouldAnnotateProject(this.project)) {
      return base;
    }

    const section = formatMigrationSection(listNewMigrationFiles(runGit));
    return injectMigrationSection(base, section);
  }
};
