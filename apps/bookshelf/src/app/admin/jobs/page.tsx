"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { JobStatus, TelegramBotStatus } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { ScheduledTaskCard } from "@/components/admin/ScheduledTaskCard";

const MONTHLY_DIGEST_ENABLED_KEY = "monthly_digest_enabled";

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
  "cover-backfill": {
    label: "Cover Backfill",
    description:
      "Looks up a cover from Open Library/Google Books for any book that still has none. Runs automatically on the configured interval.",
  },
  "registration-prune": {
    label: "Registration Prune",
    description:
      "Deletes abandoned signups that never submitted their verification code. Runs automatically on the configured interval.",
  },
  "monthly-digest": {
    label: "Monthly Digest",
    description:
      "Sends a monthly email (plus a Telegram push for members who've linked and enabled that channel) to opted-in members with new books and top recommendations from the previous calendar month.",
  },
  "due-date-reminder": {
    label: "Due-Date Reminder",
    description:
      "Notifies borrowers whose loan is due back soon (in-app always, plus email/Telegram per their own notification preferences). Runs automatically on the configured interval.",
  },
  backup: {
    label: "Backup",
    description:
      "Creates a compressed snapshot of the database and cover images, pruning old snapshots beyond the retention count. Runs automatically on the configured interval.",
  },
};

const INTERVAL_PRESETS = ["1h", "6h", "12h", "24h", "48h", "168h", "720h"];
const INTERVAL_LABELS: Record<string, string> = {
  "1h": "Every hour",
  "6h": "Every 6 hours",
  "12h": "Every 12 hours",
  "24h": "Every 24 hours",
  "48h": "Every 2 days",
  "168h": "Every week",
  "720h": "Every month",
};

const JOB_SETTING_KEYS: Record<string, string> = {
  "cover-refresh": "cover_refresh_interval",
  "description-reconciliation": "description_reconciliation_interval",
  "cover-backfill": "cover_backfill_interval",
  "registration-prune": "registration_prune_interval",
  "monthly-digest": "monthly_digest_interval",
  "due-date-reminder": "due_date_reminder_interval",
};

export default function AdminJobsPage() {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [telegramBotStatus, setTelegramBotStatus] =
    useState<TelegramBotStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [triggering, setTriggering] = useState<string | null>(null);
  const [sendingTestEmail, setSendingTestEmail] = useState(false);
  const [sendingTestTelegram, setSendingTestTelegram] = useState(false);
  const [digestEnabled, setDigestEnabled] = useState(true);
  const [savingDigestEnabled, setSavingDigestEnabled] = useState(false);

  const loadJobs = useCallback(async () => {
    try {
      const [data, settings] = await Promise.all([
        api.adminGetJobs(),
        api.adminGetSettings(),
      ]);
      setJobs(data);
      const setting = settings.find(
        (s) => s.key === MONTHLY_DIGEST_ENABLED_KEY,
      );
      if (setting) setDigestEnabled(setting.value === "true");
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load jobs");
    } finally {
      setLoading(false);
    }

    // Deliberately outside the try/catch above and its own call, not part
    // of the Promise.all: this is a "nice to have" status indicator, not
    // core job data — a failure here (network hiccup, backend that hasn't
    // picked up this route yet) must never block the actual jobs list from
    // rendering. Silently leaves telegramBotStatus at its previous value on
    // failure rather than surfacing a second error banner for a secondary
    // indicator.
    api
      .adminTelegramBotStatus()
      .then(setTelegramBotStatus)
      .catch(() => {});
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
      interval = setInterval(loadJobs, 60_000);
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

  async function handleToggleDigestEnabled(checked: boolean) {
    setSavingDigestEnabled(true);
    const previous = digestEnabled;
    setDigestEnabled(checked);
    try {
      await api.adminUpdateSettings([
        { key: MONTHLY_DIGEST_ENABLED_KEY, value: checked ? "true" : "false" },
      ]);
      toast.success(
        checked ? "Monthly digest enabled" : "Monthly digest disabled",
      );
    } catch (err) {
      setDigestEnabled(previous);
      toast.error(
        err instanceof Error ? err.message : "Failed to update monthly digest",
      );
    } finally {
      setSavingDigestEnabled(false);
    }
  }

  async function handleDigestTestEmail() {
    setSendingTestEmail(true);
    try {
      const result = await api.adminDigestTestEmail();
      toast.success(`Test email sent to ${result.recipient}`);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to send test email",
      );
    } finally {
      setSendingTestEmail(false);
    }
  }

  async function handleDigestTestTelegram() {
    setSendingTestTelegram(true);
    try {
      await api.adminDigestTestTelegram();
      toast.success("Test Telegram message sent");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to send test message",
      );
    } finally {
      setSendingTestTelegram(false);
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

      {telegramBotStatus?.configured && (
        <div className="flex items-center justify-between rounded-lg border p-4">
          <div>
            <p className="text-sm font-medium">Telegram bot</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Whether the apps/bookshelf-bot process is up and polling for
              /start commands — separate from whether push notifications are
              being delivered.
            </p>
          </div>
          <Badge variant={telegramBotStatus.online ? "success" : "destructive"}>
            {telegramBotStatus.online ? "Online" : "Offline"}
          </Badge>
        </div>
      )}

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
            >
              {job.name === "monthly-digest" && (
                <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={digestEnabled}
                      onCheckedChange={handleToggleDigestEnabled}
                      disabled={savingDigestEnabled}
                      aria-label="Enable monthly digest"
                    />
                    <span className="text-sm">
                      {digestEnabled ? "Enabled" : "Disabled"}
                    </span>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleDigestTestEmail}
                      disabled={sendingTestEmail}
                    >
                      {sendingTestEmail ? "Sending…" : "Send test email"}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleDigestTestTelegram}
                      disabled={sendingTestTelegram}
                    >
                      {sendingTestTelegram ? "Sending…" : "Send test Telegram"}
                    </Button>
                  </div>
                </div>
              )}
            </ScheduledTaskCard>
          );
        })}
      </div>
    </div>
  );
}
