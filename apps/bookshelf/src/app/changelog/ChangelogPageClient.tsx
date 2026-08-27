"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";

import type { ChangelogEntry } from "@/lib/changelog";
import { formatChangelogDateLabel } from "@/lib/changelog";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { ChangelogEmptyState } from "@/app/changelog/ChangelogEmptyState";
import { ChangelogHeroRelease } from "@/app/changelog/ChangelogHeroRelease";
import { ChangelogOpsCard } from "@/app/changelog/ChangelogOpsCard";
import { ChangelogTimelineEntry } from "@/app/changelog/ChangelogTimelineEntry";
import { cn } from "@/lib/utils";

const RECENT_TIMELINE_COUNT = 2;

type ChangelogPageClientProps = {
  entries: ChangelogEntry[];
  appVersion: string;
};

export function ChangelogPageClient({
  entries,
  appVersion,
}: ChangelogPageClientProps) {
  const heroRef = useRef<HTMLDivElement>(null);
  const [stickyVisible, setStickyVisible] = useState(false);
  const [isLoggedIn] = useState(
    () =>
      typeof window !== "undefined" &&
      Boolean(localStorage.getItem("bookshelf_token")),
  );
  const [showAdminDetails, setShowAdminDetails] = useState(
    () =>
      typeof window !== "undefined" &&
      Boolean(localStorage.getItem("bookshelf_token")),
  );

  useEffect(() => {
    const hero = heroRef.current;
    if (!hero) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        setStickyVisible(!entry.isIntersecting);
      },
      { rootMargin: "-64px 0px 0px 0px", threshold: 0 },
    );

    observer.observe(hero);
    return () => observer.disconnect();
  }, [entries.length]);

  if (entries.length === 0) {
    return (
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-8">
        <ChangelogPageHeader
          showAdminDetails={showAdminDetails}
          onAdminDetailsChange={setShowAdminDetails}
          isLoggedIn={isLoggedIn}
        />
        <ChangelogEmptyState />
        <ChangelogFooterLinks />
      </div>
    );
  }

  const [latest, ...rest] = entries;
  const recentTimeline = rest.slice(0, RECENT_TIMELINE_COUNT);
  const olderEntries = rest.slice(RECENT_TIMELINE_COUNT);
  const stickyDate = formatChangelogDateLabel(latest.date, {
    preferRelative: true,
  });

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-8">
      <div
        className={cn(
          "sticky top-0 z-10 -mx-4 border-b bg-background/95 px-4 py-3 backdrop-blur supports-[backdrop-filter]:bg-background/80 transition-opacity md:top-0",
          stickyVisible
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0",
        )}
        aria-hidden={!stickyVisible}
      >
        <div className="mx-auto flex max-w-2xl items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold">Changelog</p>
            <p className="truncate text-xs text-muted-foreground">
              v{latest.version}
              {stickyDate ? ` · ${stickyDate}` : ""}
            </p>
          </div>
        </div>
      </div>

      <ChangelogPageHeader
        showAdminDetails={showAdminDetails}
        onAdminDetailsChange={setShowAdminDetails}
        isLoggedIn={isLoggedIn}
      />

      <ChangelogOpsCard appVersion={appVersion} entries={entries} />

      <div ref={heroRef}>
        <ChangelogHeroRelease
          entry={latest}
          showAdminDetails={showAdminDetails}
        />
      </div>

      {recentTimeline.length > 0 || olderEntries.length > 0 ? (
        <>
          <Separator />
          <div className="flex flex-col gap-2">
            <h2 className="text-lg font-semibold">Previous releases</h2>
            <div className="flex flex-col">
              {recentTimeline.map((entry, index) => (
                <ChangelogTimelineEntry
                  key={entry.version}
                  entry={entry}
                  showAdminDetails={showAdminDetails}
                  preferRelativeDate={index < 2}
                  isLast={
                    olderEntries.length === 0 &&
                    index === recentTimeline.length - 1
                  }
                />
              ))}
            </div>

            {olderEntries.length > 0 ? (
              <Accordion type="single" collapsible className="mt-2">
                <AccordionItem value="older" className="border-none">
                  <AccordionTrigger className="rounded-lg border px-4 py-3 hover:no-underline">
                    Show {olderEntries.length} older release
                    {olderEntries.length === 1 ? "" : "s"}
                  </AccordionTrigger>
                  <AccordionContent className="pt-4 pb-0">
                    <div className="flex flex-col">
                      {olderEntries.map((entry, index) => (
                        <ChangelogTimelineEntry
                          key={entry.version}
                          entry={entry}
                          showAdminDetails={showAdminDetails}
                          isLast={index === olderEntries.length - 1}
                        />
                      ))}
                    </div>
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
            ) : null}
          </div>
        </>
      ) : null}

      <ChangelogFooterLinks />
    </div>
  );
}

function ChangelogPageHeader({
  showAdminDetails,
  onAdminDetailsChange,
  isLoggedIn,
}: {
  showAdminDetails: boolean;
  onAdminDetailsChange: (value: boolean) => void;
  isLoggedIn: boolean;
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h1 className="text-3xl font-bold">Changelog</h1>
        <p className="text-muted-foreground">
          What&apos;s new in Bookshelf — release notes for members and admins.
        </p>
      </div>

      <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/30 px-4 py-3">
        <div className="min-w-0">
          <Label htmlFor="admin-details" className="text-sm font-medium">
            Admin details
          </Label>
          <p className="text-xs text-muted-foreground">
            {isLoggedIn
              ? "Show database migrations and PR links."
              : "Sign in to see deployment status; enable for migrations and PR links."}
          </p>
        </div>
        <Switch
          id="admin-details"
          checked={showAdminDetails}
          onCheckedChange={onAdminDetailsChange}
        />
      </div>
    </div>
  );
}

function ChangelogFooterLinks() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
      <Link href="/catalog" className="hover:underline underline-offset-2">
        Catalog
      </Link>
      <span aria-hidden>·</span>
      <Link href="/about" className="hover:underline underline-offset-2">
        About
      </Link>
    </div>
  );
}
