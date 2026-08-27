"use client";

import Link from "next/link";
import { FileWarning } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function ChangelogEmptyState() {
  return (
    <Card className="border-dashed">
      <CardHeader className="items-center text-center">
        <div className="flex size-12 items-center justify-center rounded-full bg-muted">
          <FileWarning className="size-6 text-muted-foreground" />
        </div>
        <CardTitle>Release notes unavailable</CardTitle>
        <CardDescription className="max-w-md text-balance">
          Bookshelf couldn&apos;t load its changelog. This usually means the
          build step that embeds release notes didn&apos;t run — try redeploying
          or contact your admin.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center justify-center gap-2">
        <Button asChild variant="default">
          <Link href="/catalog">Back to catalog</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/about">About Bookshelf</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
