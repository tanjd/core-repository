"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import Markdown from "react-markdown";
import { Bandage, Database, Rocket, Sparkles } from "lucide-react";

import {
  filterSectionsForAudience,
  parseChangelogSections,
  sanitizeChangelogMarkdown,
  type ChangelogSectionKind,
} from "@/lib/changelog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const SECTION_META: Record<
  ChangelogSectionKind,
  {
    label: string;
    badgeVariant: "success" | "secondary" | "outline" | "default";
    icon?: ReactNode;
  } | null
> = {
  features: {
    label: "Features",
    badgeVariant: "success",
    icon: <Rocket className="size-3" />,
  },
  fixes: {
    label: "Fixes",
    badgeVariant: "secondary",
    icon: <Bandage className="size-3" />,
  },
  migrations: {
    label: "Database migrations",
    badgeVariant: "outline",
    icon: <Database className="size-3" />,
  },
  thank_you: null,
  other: {
    label: "Changes",
    badgeVariant: "default",
    icon: <Sparkles className="size-3" />,
  },
};

function MarkdownContent({
  content,
  showAdminDetails,
}: {
  content: string;
  showAdminDetails: boolean;
}) {
  const markdown = sanitizeChangelogMarkdown(content, { showAdminDetails });

  return (
    <div
      className={cn(
        "prose prose-neutral dark:prose-invert max-w-none",
        "prose-p:my-2 prose-p:leading-relaxed",
        "prose-ul:my-2 prose-li:my-1",
        "[&_a]:text-primary [&_a]:underline-offset-2 [&_a:hover]:underline",
        !showAdminDetails &&
          "[&_a]:pointer-events-none [&_a]:text-muted-foreground [&_a]:no-underline",
      )}
    >
      <Markdown
        components={{
          a: ({ href, children }) => (
            <Link
              href={href ?? "#"}
              target={href?.startsWith("http") ? "_blank" : undefined}
              rel={href?.startsWith("http") ? "noopener noreferrer" : undefined}
            >
              {children}
            </Link>
          ),
        }}
      >
        {markdown}
      </Markdown>
    </div>
  );
}

export function ChangelogEntryBody({
  body,
  showAdminDetails,
  className,
}: {
  body: string;
  showAdminDetails: boolean;
  className?: string;
}) {
  const sections = filterSectionsForAudience(parseChangelogSections(body), {
    showAdminDetails,
  });

  if (sections.length === 0) {
    return (
      <p className={cn("text-sm text-muted-foreground", className)}>
        No user-facing notes for this release.
      </p>
    );
  }

  return (
    <div className={cn("flex flex-col gap-4", className)}>
      {sections.map((section) => {
        const meta = SECTION_META[section.kind];

        if (section.kind === "migrations") {
          return (
            <Alert key={section.title} variant="info">
              <Database />
              <AlertTitle>{meta?.label ?? section.title}</AlertTitle>
              <AlertDescription>
                <MarkdownContent
                  content={section.content}
                  showAdminDetails={showAdminDetails}
                />
              </AlertDescription>
            </Alert>
          );
        }

        return (
          <div key={section.title} className="flex flex-col gap-2">
            {meta ? (
              <Badge variant={meta.badgeVariant} className="w-fit">
                {meta.icon}
                {meta.label}
              </Badge>
            ) : null}
            <MarkdownContent
              content={section.content}
              showAdminDetails={showAdminDetails}
            />
          </div>
        );
      })}
    </div>
  );
}
