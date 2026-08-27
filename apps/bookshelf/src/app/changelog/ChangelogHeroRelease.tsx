"use client";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import {
  extractTeaserLines,
  formatChangelogDateLabel,
  type ChangelogEntry,
} from "@/lib/changelog";
import { ChangelogEntryBody } from "@/app/changelog/ChangelogEntryBody";
import {
  ChangelogVersionHeader,
  CurrentReleaseBadge,
} from "@/app/changelog/ChangelogVersionHeader";

export function ChangelogHeroRelease({
  entry,
  showAdminDetails,
}: {
  entry: ChangelogEntry;
  showAdminDetails: boolean;
}) {
  const teasers = extractTeaserLines(entry, 2);
  const dateLabel = formatChangelogDateLabel(entry.date, {
    preferRelative: true,
  });

  return (
    <Card className="gap-4 border-l-4 border-l-primary py-4">
      <CardHeader className="gap-3 px-4 pb-0">
        <ChangelogVersionHeader
          version={entry.version}
          date={entry.date}
          preferRelativeDate
          badge={<CurrentReleaseBadge />}
        />
        {teasers.length > 0 ? (
          <ul className="list-none space-y-1.5 text-base text-foreground">
            {teasers.map((line) => (
              <li key={line} className="flex gap-2">
                <span aria-hidden className="text-primary">
                  •
                </span>
                <span>{line}</span>
              </li>
            ))}
          </ul>
        ) : (
          <CardDescription>
            {dateLabel
              ? `Released ${dateLabel.toLowerCase()}.`
              : "Latest Bookshelf release."}
          </CardDescription>
        )}
      </CardHeader>
      <CardContent className="px-4 pt-0">
        <ChangelogEntryBody
          body={entry.body}
          showAdminDetails={showAdminDetails}
        />
      </CardContent>
    </Card>
  );
}
