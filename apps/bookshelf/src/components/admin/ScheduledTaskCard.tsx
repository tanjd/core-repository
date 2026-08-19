"use client";

import { useState, type ReactNode } from "react";
import { Play, Clock } from "lucide-react";
import { toast } from "sonner";
import type { JobStatus } from "@/lib/types";
import { timeAgo, timeUntil } from "@/lib/timeFormat";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

// Go duration format, e.g. "24h", "6h30m". Matches both a single unit
// ("24h") and a compact h/m/s combination ("6h30m").
function isValidGoDuration(value: string): boolean {
  return (
    /^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)+$/.test(value) ||
    /^(\d+h)?(\d+m)?(\d+s)?$/.test(value)
  );
}

export interface ScheduledTaskCardProps {
  title: string;
  description?: string;
  status: JobStatus | null;
  onRunNow: () => Promise<void>;
  triggering: boolean;
  intervalPresets: string[];
  intervalLabels: Record<string, string>;
  onSaveInterval: (value: string) => Promise<void>;
  /** Extra rows rendered below the interval row, e.g. a retention setting. */
  children?: ReactNode;
}

/**
 * A "scheduled task" status card — shared shape for admin pages that expose
 * a background job's run state and let an admin trigger it or edit its
 * interval (Jobs, Backups). Mirrors the scheduled-tasks pattern used by
 * Jellyfin/Sonarr: status + last/next run + manual trigger + interval editor.
 */
export function ScheduledTaskCard({
  title,
  description,
  status,
  onRunNow,
  triggering,
  intervalPresets,
  intervalLabels,
  onSaveInterval,
  children,
}: ScheduledTaskCardProps) {
  const [editingInterval, setEditingInterval] = useState(false);
  const [intervalInput, setIntervalInput] = useState("");
  const [savingInterval, setSavingInterval] = useState(false);

  async function handleSaveInterval() {
    const value = intervalInput.trim();
    if (!value) return;
    if (!isValidGoDuration(value)) {
      toast.error(
        "Invalid duration. Use Go duration format, e.g. 24h, 6h, 1h30m",
      );
      return;
    }
    setSavingInterval(true);
    try {
      await onSaveInterval(value);
      setEditingInterval(false);
    } finally {
      setSavingInterval(false);
    }
  }

  return (
    <div className="rounded-lg border bg-card p-4 flex flex-col gap-4">
      {/* Header row */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-1 min-w-0">
          <div className="flex items-center gap-2">
            <p className="font-medium text-sm">{title}</p>
            {status && (
              <Badge
                variant={status.running ? "default" : "secondary"}
                className={cn(
                  "text-[10px] px-1.5 py-0",
                  status.running && "animate-pulse",
                )}
              >
                {status.running ? "Running" : "Idle"}
              </Badge>
            )}
          </div>
          {description && (
            <p className="text-xs text-muted-foreground">{description}</p>
          )}
        </div>

        <Button
          size="sm"
          variant="outline"
          onClick={onRunNow}
          disabled={!status || status.running || triggering}
          className="shrink-0"
        >
          <Play className="size-3.5" />
          {triggering ? "Queuing…" : "Run Now"}
        </Button>
      </div>

      {/* Stats row */}
      <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-muted-foreground border-t pt-3">
        <span>
          <span className="font-medium text-foreground">Last run:</span>{" "}
          {status?.last_run_at ? timeAgo(status.last_run_at) : "never"}
        </span>
        <span>
          <span className="font-medium text-foreground">Next run:</span>{" "}
          {status?.running
            ? "after this run"
            : status?.next_run_at
              ? timeUntil(status.next_run_at)
              : "pending first run"}
        </span>
        {status?.last_result && !status.running && (
          <span>
            <span className="font-medium text-foreground">Result:</span>{" "}
            {status.last_result}
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
                {status?.interval ?? "…"}
              </span>
            </span>
          </div>
          {!editingInterval && (
            <button
              onClick={() => {
                setEditingInterval(true);
                setIntervalInput(status?.interval ?? intervalPresets[0]);
              }}
              className="text-xs text-primary hover:underline"
            >
              Change
            </button>
          )}
        </div>

        {editingInterval && (
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap gap-1.5">
              {intervalPresets.map((p) => (
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
                  {intervalLabels[p] ?? p}
                </button>
              ))}
            </div>
            <div className="flex gap-2">
              <Input
                value={intervalInput}
                onChange={(e) => setIntervalInput(e.target.value)}
                placeholder="e.g. 24h, 6h, 1h30m"
                className="h-8 text-sm"
                onKeyDown={(e) => e.key === "Enter" && handleSaveInterval()}
              />
              <Button
                size="sm"
                onClick={handleSaveInterval}
                disabled={savingInterval || !intervalInput.trim()}
              >
                {savingInterval ? "Saving…" : "Save"}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setEditingInterval(false)}
              >
                Cancel
              </Button>
            </div>
          </div>
        )}
      </div>

      {children}
    </div>
  );
}
