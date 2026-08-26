"use client";

import { useEffect, useState } from "react";
import { formatSchemaMigration } from "@/lib/changelog";

type HealthResponse = {
  status: string;
  version?: string;
  schema_version?: number;
};

export function ChangelogRuntimeInfo() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
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
      .catch(() => setHealth(null));
  }, [isLoggedIn]);

  if (!isLoggedIn || !health) return null;

  const apiVersion = health.version ?? "unknown";
  const schemaVersion =
    typeof health.schema_version === "number"
      ? formatSchemaMigration(health.schema_version)
      : "unknown";

  return (
    <p className="text-sm text-muted-foreground">
      Running API {apiVersion} · schema migration {schemaVersion}
    </p>
  );
}
