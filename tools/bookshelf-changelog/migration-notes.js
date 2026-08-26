const MIGRATION_GLOB = "apps/bookshelf-backend/internal/db/migrations/*.up.sql";
const BACKEND_TAG_PREFIX = "bookshelf-backend@";
const THANK_YOU_MARKER = "### ❤️ Thank You";

const BOOKSHELF_PROJECTS = new Set(["bookshelf", "bookshelf-backend"]);

/** @param {string | null | undefined} project */
function shouldAnnotateProject(project) {
  return project != null && BOOKSHELF_PROJECTS.has(project);
}

/** @param {string[]} filePaths */
function extractMigrationIds(filePaths) {
  return filePaths
    .map((filePath) => filePath.match(/(\d{6})_/)?.[1])
    .filter((id) => id != null);
}

/**
 * @param {string[]} filePaths
 * @returns {string} Empty string when there are no migrations to document.
 */
function formatMigrationSection(filePaths) {
  const ids = extractMigrationIds(filePaths);
  if (ids.length === 0) {
    return "";
  }

  const list = ids.map((id) => `**${id}**`).join(", ");
  return [
    "### Database migrations",
    "",
    `Includes migration ${list} — automatic on startup; no manual SQL required.`,
  ].join("\n");
}

/**
 * @param {string} changelog
 * @param {string} section
 */
function injectMigrationSection(changelog, section) {
  if (!section) {
    return changelog;
  }

  if (changelog.includes(THANK_YOU_MARKER)) {
    return changelog.replace(
      THANK_YOU_MARKER,
      `${section}\n\n${THANK_YOU_MARKER}`,
    );
  }

  return `${changelog}\n\n${section}`;
}

/**
 * @param {(command: string) => string} runGit
 * @returns {string[]}
 */
function listNewMigrationFiles(runGit) {
  let previousTag;
  try {
    previousTag = runGit(
      `git describe --tags --abbrev=0 --match '${BACKEND_TAG_PREFIX}*'`,
    ).trim();
  } catch {
    return [];
  }

  if (!previousTag) {
    return [];
  }

  const diff = runGit(
    `git diff --name-only --diff-filter=A ${previousTag}..HEAD -- ${MIGRATION_GLOB}`,
  ).trim();

  if (!diff) {
    return [];
  }

  return diff.split("\n").filter(Boolean);
}

module.exports = {
  BACKEND_TAG_PREFIX,
  MIGRATION_GLOB,
  THANK_YOU_MARKER,
  extractMigrationIds,
  formatMigrationSection,
  injectMigrationSection,
  listNewMigrationFiles,
  shouldAnnotateProject,
};
