"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { RefreshCw, Play, Clock } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { JobStatus } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const JOB_META: Record<string, { label: string; description: string }> = {
  "cover-refresh": {
    label: "Cover Image Refresh",
    description:
      "Downloads and caches external book cover images locally. Runs automatically on the configured interval.",
  },
};

const INTERVAL_PRESETS = ["1h", "6h", "12h", "24h", "48h", "168h"];
const INTERVAL_LABELS: Record<string, string> = {
  "1h": "Every hour",
  "6h": "Every 6 hours",
  "12h": "Every 12 hours",
  "24h": "Every 24 hours",
  "48h": "Every 2 days",
  "168h": "Every week",
};

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export default function AdminJobsPage() {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [triggering, setTriggering] = useState<string | null>(null);
  const [editingInterval, setEditingInterval] = useState<string | null>(null);
  const [intervalInput, setIntervalInput] = useState("");
  const [savingInterval, setSavingInterval] = useState(false);

  const loadJobs = useCallback(async () => {
    try {
      const data = await api.adminGetJobs();
      setJobs(data);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    } finally {
      setLoading(false);
    }
  }, []);

  const loadedRef = useRef(false);
  useEffect(() => {
    if (!loadedRef.current) {
      loadedRef.current = true;
      loadJobs();
    }
    let interval: ReturnType<typeof setInterval> | null = null;

    function startPolling() {
      if (interval !== null) return;
      interval = setInterval(loadJobs, 3_000);
    }
    function stopPolling() {
      if (interval === null) return;
      clearInterval(interval);
      interval = null;
    }
    function handleVisibilityChange() {
      if (document.visibilityState === "visible") {
        loadJobs();
        startPolling();
      } else {
        stopPolling();
      }
    }

    startPolling();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      stopPolling();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [loadJobs]);

  async function handleRun(jobName: string) {
    setTriggering(jobName);
    try {
      await api.adminRunJob(jobName);
      toast.success("Job triggered — starting shortly.");
      setTimeout(loadJobs, 800);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to trigger job");
    } finally {
      setTriggering(null);
    }
  }

  async function handleSaveInterval(jobName: string) {
    const value = intervalInput.trim();
    if (!value) return;
    // Basic validation: must be a Go duration like "24h", "6h30m", etc.
    if (
      !/^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)+$/.test(value) &&
      !/^(\d+h)?(\d+m)?(\d+s)?$/.test(value)
    ) {
      toast.error(
        "Invalid duration. Use Go duration format, e.g. 24h, 6h, 1h30m",
      );
      return;
    }
    setSavingInterval(true);
    try {
      const settingKey =
        jobName === "cover-refresh" ? "cover_refresh_interval" : null;
      if (!settingKey) return;
      await api.adminUpdateSettings([{ key: settingKey, value }]);
      toast.success("Interval updated — takes effect within 1 minute.");
      setEditingInterval(null);
      loadJobs();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to save interval",
      );
    } finally {
      setSavingInterval(false);
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-3 ">
        {[1].map((i) => (
          <Skeleton key={i} className="h-36 rounded-lg" />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col gap-3 ">
        <p className="text-sm text-destructive">{error}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={loadJobs}
          className="self-start"
        >
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 ">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Background jobs run on a schedule. Use &quot;Run Now&quot; to trigger
          immediately.
        </p>
        <Button variant="ghost" size="sm" onClick={loadJobs}>
          <RefreshCw className="size-3.5" /> Refresh
        </Button>
      </div>

      <div className="flex flex-col gap-3">
        {jobs.map((job) => {
          const meta = JOB_META[job.name];
          const isEditingThis = editingInterval === job.name;

          return (
            <div
              key={job.name}
              className="rounded-lg border bg-card p-4 flex flex-col gap-4"
            >
              {/* Header row */}
              <div className="flex items-start justify-between gap-4">
                <div className="flex flex-col gap-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-sm">
                      {meta?.label ?? job.name}
                    </p>
                    <Badge
                      variant={job.running ? "default" : "secondary"}
                      className={cn(
                        "text-[10px] px-1.5 py-0",
                        job.running && "animate-pulse",
                      )}
                    >
                      {job.running ? "Running" : "Idle"}
                    </Badge>
                  </div>
                  {meta?.description && (
                    <p className="text-xs text-muted-foreground">
                      {meta.description}
                    </p>
                  )}
                </div>

                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => handleRun(job.name)}
                  disabled={job.running || triggering === job.name}
                  className="shrink-0"
                >
                  <Play className="size-3.5" />
                  {triggering === job.name ? "Queuing…" : "Run Now"}
                </Button>
              </div>

              {/* Stats row */}
              <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-muted-foreground border-t pt-3">
                <span>
                  <span className="font-medium text-foreground">Last run:</span>{" "}
                  {job.last_run_at ? timeAgo(job.last_run_at) : "never"}
                </span>
                {job.last_result && !job.running && (
                  <span>
                    <span className="font-medium text-foreground">Result:</span>{" "}
                    {job.last_result}
                  </span>
                )}
              </div>

              {/* Interval row */}
              <div className="flex flex-col gap-2 border-t pt-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Clock className="size-3.5" />
                    <span>
                      Runs every{" "}
                      <span className="font-medium text-foreground">
                        {job.interval}
                      </span>
                    </span>
                  </div>
                  {!isEditingThis && (
                    <button
                      onClick={() => {
                        setEditingInterval(job.name);
                        setIntervalInput(job.interval);
                      }}
                      className="text-xs text-primary hover:underline"
                    >
                      Change
                    </button>
                  )}
                </div>

                {isEditingThis && (
                  <div className="flex flex-col gap-2">
                    <div className="flex flex-wrap gap-1.5">
                      {INTERVAL_PRESETS.map((p) => (
                        <button
                          key={p}
                          onClick={() => setIntervalInput(p)}
                          className={cn(
                            "px-2 py-0.5 rounded text-xs border transition-colors",
                            intervalInput === p
                              ? "bg-primary text-primary-foreground border-primary"
                              : "hover:bg-accent",
                          )}
                        >
                          {INTERVAL_LABELS[p] ?? p}
                        </button>
                      ))}
                    </div>
                    <div className="flex gap-2">
                      <Input
                        value={intervalInput}
                        onChange={(e) => setIntervalInput(e.target.value)}
                        placeholder="e.g. 24h, 6h, 1h30m"
                        className="h-8 text-sm"
                        onKeyDown={(e) =>
                          e.key === "Enter" && handleSaveInterval(job.name)
                        }
                      />
                      <Button
                        size="sm"
                        onClick={() => handleSaveInterval(job.name)}
                        disabled={savingInterval || !intervalInput.trim()}
                      >
                        {savingInterval ? "Saving…" : "Save"}
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => setEditingInterval(null)}
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
