export type ChangelogEntry = {
  version: string;
  date: string | null;
  body: string;
};

const VERSION_HEADER =
  /^#{1,2}\s+(?:\[?v?(\d+\.\d+\.\d+)\]?(?:\s*\(([^)]+)\))?.*)$/;

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
