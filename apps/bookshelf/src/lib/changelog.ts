export type ChangelogEntry = {
  version: string;
  date: string | null;
  body: string;
};

export type ChangelogSectionKind =
  | "features"
  | "fixes"
  | "migrations"
  | "thank_you"
  | "other";

export type ChangelogSection = {
  kind: ChangelogSectionKind;
  title: string;
  content: string;
};

const VERSION_HEADER =
  /^#{1,2}\s+(?:\[?v?(\d+\.\d+\.\d+)\]?(?:\s*\(([^)]+)\))?.*)$/;

const SECTION_HEADER = /^###\s+(.+)$/;

const PR_LINK_PATTERN = /\s*\(\[#\d+\]\([^)]+\)\)/g;

const MIGRATION_NUMBER_PATTERN = /migration\s+\*\*(\d+)\*\*/i;

/** Returns true when `a` is a higher semver than `b` (major.minor.patch only). */
export function semverGt(a: string, b: string): boolean {
  const parse = (value: string) => value.split(".").map((part) => Number(part));
  const [aMajor, aMinor, aPatch] = parse(a);
  const [bMajor, bMinor, bPatch] = parse(b);

  if (aMajor !== bMajor) return aMajor > bMajor;
  if (aMinor !== bMinor) return aMinor > bMinor;
  return aPatch > bPatch;
}

/** Parses nx-release-style CHANGELOG.md into newest-first version sections. */
export function parseChangelogMarkdown(
  content: string,
  maxVersions = 15,
): ChangelogEntry[] {
  const lines = content.split("\n");
  const entries: ChangelogEntry[] = [];
  let current: ChangelogEntry | null = null;
  const bodyLines: string[] = [];

  const pushCurrent = () => {
    if (!current) return;
    current.body = bodyLines.join("\n").trim();
    entries.push(current);
    current = null;
    bodyLines.length = 0;
  };

  for (const line of lines) {
    const match = line.match(VERSION_HEADER);
    if (match) {
      pushCurrent();
      if (entries.length >= maxVersions) break;
      current = {
        version: match[1],
        date: match[2]?.trim() ?? null,
        body: "",
      };
      continue;
    }

    if (current) {
      bodyLines.push(line);
    }
  }

  pushCurrent();
  return entries;
}

export function formatSchemaMigration(version: number): string {
  return String(version).padStart(6, "0");
}

export function versionAnchorId(version: string): string {
  return `v${version}`;
}

export function classifySectionTitle(title: string): ChangelogSectionKind {
  const normalized = title.toLowerCase();

  if (normalized.includes("thank you")) return "thank_you";
  if (normalized.includes("database migration")) return "migrations";
  if (normalized.includes("feature")) return "features";
  if (normalized.includes("fix")) return "fixes";

  return "other";
}

/** Splits a release body into logical sections keyed by ### headings. */
export function parseChangelogSections(body: string): ChangelogSection[] {
  const lines = body.split("\n");
  const sections: ChangelogSection[] = [];
  let current: ChangelogSection | null = null;
  const contentLines: string[] = [];

  const pushCurrent = () => {
    if (!current) return;
    current.content = contentLines.join("\n").trim();
    if (current.content.length > 0) {
      sections.push(current);
    }
    current = null;
    contentLines.length = 0;
  };

  for (const line of lines) {
    const match = line.match(SECTION_HEADER);
    if (match) {
      pushCurrent();
      const title = match[1].trim();
      current = {
        kind: classifySectionTitle(title),
        title,
        content: "",
      };
      continue;
    }

    if (current) {
      contentLines.push(line);
    }
  }

  pushCurrent();
  return sections;
}

export function stripPrLinks(text: string): string {
  return text.replace(PR_LINK_PATTERN, "").trim();
}

export function stripMarkdownInline(text: string): string {
  return stripPrLinks(text)
    .replace(/\*\*(.+?)\*\*/g, "$1")
    .replace(/\[(.+?)\]\([^)]+\)/g, "$1")
    .replace(/^-\s+/, "")
    .trim();
}

/** Returns short teaser lines suitable for banners and hero summaries. */
export function extractTeaserLines(
  entry: ChangelogEntry,
  maxLines = 2,
): string[] {
  const teasers: string[] = [];

  for (const section of parseChangelogSections(entry.body)) {
    if (section.kind === "thank_you" || section.kind === "migrations") {
      continue;
    }

    for (const line of section.content.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed.startsWith("-")) continue;

      const plain = stripMarkdownInline(trimmed);
      if (plain.length === 0) continue;

      teasers.push(plain);
      if (teasers.length >= maxLines) {
        return teasers;
      }
    }
  }

  return teasers;
}

export function extractMigrationNumber(text: string): number | null {
  const match = text.match(MIGRATION_NUMBER_PATTERN);
  if (!match) return null;
  return Number.parseInt(match[1], 10);
}

/** Finds the highest migration number referenced across changelog entries. */
export function latestReferencedMigration(
  entries: ChangelogEntry[],
): number | null {
  let latest: number | null = null;

  for (const entry of entries) {
    for (const section of parseChangelogSections(entry.body)) {
      if (section.kind !== "migrations") continue;

      const migration = extractMigrationNumber(section.content);
      if (migration === null) continue;
      if (latest === null || migration > latest) {
        latest = migration;
      }
    }
  }

  return latest;
}

export function formatChangelogDate(date: string | null): string | null {
  if (!date) return null;

  const parsed = new Date(`${date}T12:00:00`);
  if (Number.isNaN(parsed.getTime())) return date;

  return parsed.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function formatRelativeChangelogDate(
  date: string | null,
): string | null {
  if (!date) return null;

  const parsed = new Date(`${date}T12:00:00`);
  if (Number.isNaN(parsed.getTime())) return null;

  const diffMs = Date.now() - parsed.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays <= 0) return "Today";
  if (diffDays === 1) return "Yesterday";
  if (diffDays < 7) return `${diffDays} days ago`;
  if (diffDays < 14) return "1 week ago";
  if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;

  return formatChangelogDate(date);
}

export function formatChangelogDateLabel(
  date: string | null,
  { preferRelative = false }: { preferRelative?: boolean } = {},
): string | null {
  if (!date) return null;

  if (preferRelative) {
    return formatRelativeChangelogDate(date) ?? formatChangelogDate(date);
  }

  return formatChangelogDate(date);
}

export function sanitizeChangelogMarkdown(
  markdown: string,
  { showAdminDetails }: { showAdminDetails: boolean },
): string {
  if (showAdminDetails) {
    return markdown;
  }

  return stripPrLinks(markdown);
}

export function filterSectionsForAudience(
  sections: ChangelogSection[],
  { showAdminDetails }: { showAdminDetails: boolean },
): ChangelogSection[] {
  return sections.filter((section) => {
    if (section.kind === "thank_you") return false;
    if (!showAdminDetails && section.kind === "migrations") return false;
    return true;
  });
}

/** Teaser for the latest release — used by upgrade notices. */
export function getLatestChangelogTeaser(
  entries: ChangelogEntry[],
): string | null {
  if (entries.length === 0) return null;
  const [firstLine] = extractTeaserLines(entries[0], 1);
  return firstLine ?? null;
}
