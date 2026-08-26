"use client";

import { useEffect, useState } from "react";
import { CheckCircle2, Database, Loader2, Server } from "lucide-react";

import {
  formatSchemaMigration,
  latestReferencedMigration,
  type ChangelogEntry,
} from "@/lib/changelog";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

type HealthResponse = {
  status: string;
  version?: string;
  schema_version?: number;
};

type ChangelogOpsCardProps = {
  appVersion: string;
  entries: ChangelogEntry[];
};

export function ChangelogOpsCard({
  appVersion,
  entries,
}: ChangelogOpsCardProps) {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [isLoggedIn] = useState(
    () =>
      typeof window !== "undefined" &&
      Boolean(localStorage.getItem("bookshelf_token")),
  );

  useEffect(() => {
    if (!isLoggedIn) return;

    fetch("/api/health")
      .then((res) => (res.ok ? res.json() : null))
      .then((data: HealthResponse | null) => setHealth(data))
      .catch(() => setHealth(null))
      .finally(() => setLoading(false));
  }, [isLoggedIn]);

  if (!isLoggedIn) return null;

  const expectedMigration = latestReferencedMigration(entries);
  const schemaVersion =
    typeof health?.schema_version === "number"
      ? formatSchemaMigration(health.schema_version)
      : null;
  const schemaMatches =
    expectedMigration !== null &&
    typeof health?.schema_version === "number" &&
    health.schema_version === expectedMigration;

  return (
    <Card className="gap-4 py-4">
      <CardHeader className="gap-1 px-4 pb-0">
        <CardTitle className="text-base">Deployment status</CardTitle>
        <CardDescription>
          Live versions from your running stack — useful after pulling new
          images.
        </CardDescription>
      </CardHeader>
      <CardContent className="px-4">
        {loading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            Checking API health…
          </div>
        ) : (
          <dl className="grid gap-3 text-sm sm:grid-cols-3">
            <div className="flex items-start gap-2">
              <Server className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <div>
                <dt className="text-muted-foreground">App</dt>
                <dd className="font-medium">v{appVersion}</dd>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <Server className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <div>
                <dt className="text-muted-foreground">API</dt>
                <dd className="font-medium">
                  {health?.version ?? "unavailable"}
                </dd>
              </div>
            </div>
            <div className="flex items-start gap-2">
              <Database className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <div>
                <dt className="text-muted-foreground">Schema</dt>
                <dd className="flex items-center gap-1.5 font-medium">
                  <span>{schemaVersion ?? "unavailable"}</span>
                  {schemaMatches ? (
                    <CheckCircle2
                      className={cn("size-4 text-success")}
                      aria-label="Schema matches latest migration"
                    />
                  ) : null}
                </dd>
              </div>
            </div>
          </dl>
        )}
      </CardContent>
    </Card>
  );
}
