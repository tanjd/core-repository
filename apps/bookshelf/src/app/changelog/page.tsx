import { ChangelogPageClient } from "@/app/changelog/ChangelogPageClient";
import { getAppVersion } from "@/lib/changelog";
import { CHANGELOG_ENTRIES } from "@/lib/changelog.generated";

export const metadata = {
  title: "Changelog — Bookshelf",
  description: "Release notes for Bookshelf",
};

export default function ChangelogPage() {
  const appVersion = getAppVersion();

  return (
    <ChangelogPageClient entries={CHANGELOG_ENTRIES} appVersion={appVersion} />
  );
}
