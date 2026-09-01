"use client";

import Link from "next/link";
import { ArrowLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";

// Shared "‹ Back to X › Current page" trail used by every page that isn't
// reachable at more than one nesting depth from its parent — see
// apps/bookshelf/CLAUDE.md's frontend design conventions. catalog/[bookId]
// is the one exception with a variable origin (Catalog or My Books can both
// link here), so it keeps its own dynamic back target instead of using this
// component. `back` takes either a real URL (rendered as a Link, for pages
// reached via navigation) or an onClick (for a client-only step/wizard like
// share/page.tsx, where "back" just swaps local view state, not the route).
type BreadcrumbBack = { href: string } | { onClick: () => void };

export function Breadcrumb({
  back,
  backLabel,
  current,
}: {
  back: BreadcrumbBack;
  backLabel: string;
  current: string;
}) {
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-1 text-sm text-muted-foreground -ml-1"
    >
      {"href" in back ? (
        <Button variant="ghost" size="sm" asChild className="h-8 px-2">
          <Link href={back.href} aria-label={`Back to ${backLabel}`}>
            <ArrowLeft className="size-4" />
            <span>{backLabel}</span>
          </Link>
        </Button>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="h-8 px-2"
          onClick={back.onClick}
          aria-label={`Back to ${backLabel}`}
        >
          <ArrowLeft className="size-4" />
          <span>{backLabel}</span>
        </Button>
      )}
      <ChevronRight className="size-3.5 shrink-0" aria-hidden="true" />
      <span
        className="truncate text-foreground"
        title={current}
        aria-current="page"
      >
        {current}
      </span>
    </nav>
  );
}
