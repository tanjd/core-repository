"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { JobStatus } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ScheduledTaskCard } from "@/components/admin/ScheduledTaskCard";

const JOB_META: Record<string, { label: string; description: string }> = {
  "cover-refresh": {
    label: "Cover Image Refresh",
    description:
      "Downloads and caches external book cover images locally. Runs automatically on the configured interval.",
  },
  "description-reconciliation": {
    label: "Description Reconciliation",
    description:
      "Fills in missing book descriptions from other editions of the same book. Runs automatically on the configured interval.",
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

const JOB_SETTING_KEYS: Record<string, string> = {
  "cover-refresh": "cover_refresh_interval",
  "description-reconciliation": "description_reconciliation_interval",
};

export default function AdminJobsPage() {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [triggering, setTriggering] = useState<string | null>(null);

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

  async function handleSaveInterval(jobName: string, value: string) {
    const settingKey = JOB_SETTING_KEYS[jobName];
    if (!settingKey) return;
    try {
      await api.adminUpdateSettings([{ key: settingKey, value }]);
      toast.success("Interval updated — takes effect within 1 minute.");
      loadJobs();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to save interval",
      );
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
          return (
            <ScheduledTaskCard
              key={job.name}
              title={meta?.label ?? job.name}
              description={meta?.description}
              status={job}
              onRunNow={() => handleRun(job.name)}
              triggering={triggering === job.name}
              intervalPresets={INTERVAL_PRESETS}
              intervalLabels={INTERVAL_LABELS}
              onSaveInterval={(value) => handleSaveInterval(job.name, value)}
            />
          );
        })}
      </div>
    </div>
  );
}
