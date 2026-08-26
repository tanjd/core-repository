"use client";

import { cn } from "@/lib/utils";
import type { ChangelogEntry } from "@/lib/changelog";
import { ChangelogEntryBody } from "@/app/changelog/ChangelogEntryBody";
import { ChangelogVersionHeader } from "@/app/changelog/ChangelogVersionHeader";

export function ChangelogTimelineEntry({
  entry,
  showAdminDetails,
  isLast = false,
  preferRelativeDate = false,
}: {
  entry: ChangelogEntry;
  showAdminDetails: boolean;
  isLast?: boolean;
  preferRelativeDate?: boolean;
}) {
  return (
    <article className="relative flex gap-4 md:gap-5">
      <div
        className="hidden shrink-0 flex-col items-center pt-1.5 md:flex"
        aria-hidden
      >
        <span className="size-2.5 rounded-full border-2 border-primary bg-background" />
        {!isLast ? (
          <span className="mt-1 w-px flex-1 bg-border min-h-[calc(100%-0.75rem)]" />
        ) : null}
      </div>

      <div
        className={cn(
          "min-w-0 flex-1 pb-8",
          !isLast && "border-b border-border/70 md:border-b-0",
        )}
      >
        <ChangelogVersionHeader
          version={entry.version}
          date={entry.date}
          preferRelativeDate={preferRelativeDate}
          className="mb-3"
        />
        <ChangelogEntryBody
          body={entry.body}
          showAdminDetails={showAdminDetails}
        />
      </div>
    </article>
  );
}
