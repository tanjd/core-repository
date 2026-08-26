import { ChangelogPageClient } from "@/app/changelog/ChangelogPageClient";
import { CHANGELOG_ENTRIES } from "@/lib/changelog.generated";

export const metadata = {
  title: "Changelog — Bookshelf",
  description: "Release notes for Bookshelf",
};

export default function ChangelogPage() {
  const appVersion = process.env.NEXT_PUBLIC_VERSION ?? "unknown";

  return (
    <ChangelogPageClient entries={CHANGELOG_ENTRIES} appVersion={appVersion} />
  );
}
