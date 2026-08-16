"use client";

import { useEffect, useState, useCallback } from "react";
import { RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import type { MetadataProviderStatus } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

const PROVIDER_META: Record<string, { label: string; description: string }> = {
  openlibrary: {
    label: "Open Library",
    description:
      "Free, open book catalog. Always enabled. Provides title, author, ISBN, and cover images.",
  },
  google_books: {
    label: "Google Books",
    description:
      "Google's book database. Requires GOOGLE_BOOKS_API_KEY. Provides rich metadata including publisher, page count, and language.",
  },
  bookbrainz: {
    label: "BookBrainz",
    description:
      "MusicBrainz's open book database. Always enabled. No cover images.",
  },
};

export default function AdminMetadataPage() {
  const [statuses, setStatuses] = useState<MetadataProviderStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  const loadStatuses = useCallback(async () => {
    try {
      const data = await api.adminGetMetadataStatus();
      setStatuses(data);
      setError("");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load provider status",
      );
    } finally {
      setLoading(false);
      setChecking(false);
    }
  }, []);

  useEffect(() => {
    loadStatuses();
  }, [loadStatuses]);

  async function handleRefresh() {
    setChecking(true);
    await loadStatuses();
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-3">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-24 rounded-lg bg-muted animate-pulse" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col gap-3">
        <p className="text-sm text-destructive">{error}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={loadStatuses}
          className="self-start"
        >
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Live reachability check for each metadata provider. Results are not
          cached.
        </p>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleRefresh}
          disabled={checking}
        >
          <RefreshCw className={`size-3.5 ${checking ? "animate-spin" : ""}`} />
          {checking ? "Checking…" : "Refresh"}
        </Button>
      </div>

      <div className="flex flex-col gap-3">
        {statuses.map((s) => {
          const meta = PROVIDER_META[s.name];
          return (
            <div
              key={s.name}
              className="rounded-lg border bg-card p-4 flex flex-col gap-3"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <p className="font-medium text-sm">
                      {meta?.label ?? s.name}
                    </p>
                    <Badge
                      variant={s.enabled ? "secondary" : "outline"}
                      className="text-[10px] px-1.5 py-0"
                    >
                      {s.enabled ? "Enabled" : "Disabled"}
                    </Badge>
                    {s.enabled && (
                      <Badge
                        variant={s.reachable ? "success" : "destructive"}
                        className="text-[10px] px-1.5 py-0"
                      >
                        {s.reachable ? "Reachable" : "Unreachable"}
                      </Badge>
                    )}
                  </div>
                  {meta?.description && (
                    <p className="text-xs text-muted-foreground">
                      {meta.description}
                    </p>
                  )}
                </div>
                {s.enabled && s.latency_ms > 0 && (
                  <span className="text-xs text-muted-foreground shrink-0">
                    {s.latency_ms}ms
                  </span>
                )}
              </div>

              {s.error && (
                <p className="text-xs text-destructive border-t pt-2">
                  {s.error}
                </p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
