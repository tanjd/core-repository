"use client";

import { useCallback } from "react";
import { Link2 } from "lucide-react";
import { toast } from "sonner";

import {
  formatChangelogDate,
  formatChangelogDateLabel,
  versionAnchorId,
} from "@/lib/changelog";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export function ChangelogVersionHeader({
  version,
  date,
  preferRelativeDate = false,
  badge,
  className,
}: {
  version: string;
  date: string | null;
  preferRelativeDate?: boolean;
  badge?: React.ReactNode;
  className?: string;
}) {
  const anchorId = versionAnchorId(version);
  const relativeDate = formatChangelogDateLabel(date, { preferRelative: true });
  const absoluteDate = formatChangelogDate(date);

  const copyLink = useCallback(async () => {
    const url = `${window.location.origin}${window.location.pathname}#${anchorId}`;
    try {
      await navigator.clipboard.writeText(url);
      toast.success("Link copied");
    } catch {
      toast.error("Could not copy link");
    }
  }, [anchorId]);

  return (
    <div
      id={anchorId}
      className={cn(
        "group flex scroll-mt-24 flex-wrap items-center gap-x-3 gap-y-1",
        className,
      )}
    >
      <h2 className="text-xl font-semibold">v{version}</h2>
      {badge}
      {date ? (
        <time
          dateTime={date}
          className="text-sm text-muted-foreground"
          title={absoluteDate ?? undefined}
        >
          {preferRelativeDate ? relativeDate : absoluteDate}
        </time>
      ) : null}
      <button
        type="button"
        onClick={copyLink}
        className="rounded p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100"
        aria-label={`Copy link to v${version}`}
      >
        <Link2 className="size-3.5" />
      </button>
    </div>
  );
}

export function CurrentReleaseBadge() {
  return <Badge variant="default">Current release</Badge>;
}
