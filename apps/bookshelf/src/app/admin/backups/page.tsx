"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { RefreshCw, Download, Trash2, Info, MoreVertical } from "lucide-react";
import { toast } from "sonner";
import { api, downloadBackup } from "@/lib/api";
import type { JobStatus, BackupInfo } from "@/lib/types";
import { timeAgo } from "@/lib/timeFormat";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { ScheduledTaskCard } from "@/components/admin/ScheduledTaskCard";

const INTERVAL_PRESETS = ["6h", "12h", "24h", "48h", "168h"];
const INTERVAL_LABELS: Record<string, string> = {
  "6h": "Every 6 hours",
  "12h": "Every 12 hours",
  "24h": "Every 24 hours",
  "48h": "Every 2 days",
  "168h": "Every week",
};

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

export default function AdminBackupsPage() {
  const [job, setJob] = useState<JobStatus | null>(null);
  const [backups, setBackups] = useState<BackupInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [triggering, setTriggering] = useState(false);
  const [downloadingFile, setDownloadingFile] = useState<string | null>(null);

  const [editingRetention, setEditingRetention] = useState(false);
  const [retentionInput, setRetentionInput] = useState("");
  const [savingRetention, setSavingRetention] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<BackupInfo | null>(null);
  const [deleteSubmitting, setDeleteSubmitting] = useState(false);

  const [restoreDialogOpen, setRestoreDialogOpen] = useState(false);

  const loadJobAndBackups = useCallback(async () => {
    try {
      const [jobs, list] = await Promise.all([
        api.adminGetJobs(),
        api.adminListBackups(),
      ]);
      setJob(jobs.find((j) => j.name === "backup") ?? null);
      setBackups(list);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load backups");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    api.adminGetSettings().then((settings) => {
      const retention = settings.find(
        (s) => s.key === "backup_retention_count",
      );
      if (retention) setRetentionInput(retention.value);
    });
  }, []);

  const loadedRef = useRef(false);
  useEffect(() => {
    if (!loadedRef.current) {
      loadedRef.current = true;
      loadJobAndBackups();
    }
    let interval: ReturnType<typeof setInterval> | null = null;

    function startPolling() {
      if (interval !== null) return;
      interval = setInterval(loadJobAndBackups, 3_000);
    }
    function stopPolling() {
      if (interval === null) return;
      clearInterval(interval);
      interval = null;
    }
    function handleVisibilityChange() {
      if (document.visibilityState === "visible") {
        loadJobAndBackups();
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
  }, [loadJobAndBackups]);

  async function handleRunNow() {
    setTriggering(true);
    try {
      await api.adminRunJob("backup");
      toast.success("Backup triggered — starting shortly.");
      setTimeout(loadJobAndBackups, 800);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to trigger backup",
      );
    } finally {
      setTriggering(false);
    }
  }

  async function handleSaveInterval(value: string) {
    try {
      await api.adminUpdateSettings([{ key: "backup_interval", value }]);
      toast.success("Interval updated — takes effect within 1 minute.");
      loadJobAndBackups();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to save interval",
      );
    }
  }

  async function handleSaveRetention() {
    const value = retentionInput.trim();
    const n = Number(value);
    if (!value || !Number.isInteger(n) || n <= 0) {
      toast.error("Retention count must be a positive whole number");
      return;
    }
    setSavingRetention(true);
    try {
      await api.adminUpdateSettings([{ key: "backup_retention_count", value }]);
      toast.success("Retention updated.");
      setEditingRetention(false);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to save retention",
      );
    } finally {
      setSavingRetention(false);
    }
  }

  async function handleDownload(b: BackupInfo) {
    setDownloadingFile(b.filename);
    try {
      await downloadBackup(b.filename);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Download failed");
    } finally {
      setDownloadingFile(null);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleteSubmitting(true);
    try {
      await api.adminDeleteBackup(deleteTarget.filename);
      toast.success("Backup deleted");
      setDeleteTarget(null);
      loadJobAndBackups();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleteSubmitting(false);
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-40 rounded-lg" />
        <Skeleton className="h-24 rounded-lg" />
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
          onClick={loadJobAndBackups}
          className="self-start"
        >
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          Snapshots bundle the database and cover images. Restoring from one is
          a manual, on-host step —{" "}
          <button
            onClick={() => setRestoreDialogOpen(true)}
            className="text-primary hover:underline inline-flex items-center gap-1"
          >
            <Info className="size-3" />
            see how
          </button>
          .
        </p>
        <Button variant="ghost" size="sm" onClick={loadJobAndBackups}>
          <RefreshCw className="size-3.5" /> Refresh
        </Button>
      </div>

      <ScheduledTaskCard
        title="Scheduled Backups"
        description="Creates a full snapshot (DB + covers) on the configured interval, pruning older ones beyond the retention count."
        status={job}
        onRunNow={handleRunNow}
        triggering={triggering}
        intervalPresets={INTERVAL_PRESETS}
        intervalLabels={INTERVAL_LABELS}
        onSaveInterval={handleSaveInterval}
      >
        {/* Retention row */}
        <div className="flex flex-col gap-2 border-t pt-3">
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">
              Keep the newest{" "}
              <span className="font-medium text-foreground">
                {retentionInput || "…"}
              </span>{" "}
              snapshots, pruning older ones automatically
            </span>
            {!editingRetention && (
              <button
                onClick={() => setEditingRetention(true)}
                className="text-xs text-primary hover:underline"
              >
                Change
              </button>
            )}
          </div>

          {editingRetention && (
            <div className="flex gap-2">
              <Input
                type="number"
                min={1}
                value={retentionInput}
                onChange={(e) => setRetentionInput(e.target.value)}
                className="h-8 text-sm w-24"
                onKeyDown={(e) => e.key === "Enter" && handleSaveRetention()}
              />
              <Button
                size="sm"
                onClick={handleSaveRetention}
                disabled={savingRetention || !retentionInput.trim()}
              >
                {savingRetention ? "Saving…" : "Save"}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => setEditingRetention(false)}
              >
                Cancel
              </Button>
            </div>
          )}
        </div>
      </ScheduledTaskCard>

      {/* Snapshot list */}
      <div className="rounded-md border overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-3 text-left font-medium">Filename</th>
              <th className="px-4 py-3 text-left font-medium">Size</th>
              <th className="px-4 py-3 text-left font-medium">Created</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {backups.length === 0 ? (
              <tr>
                <td
                  colSpan={4}
                  className="px-4 py-6 text-center text-muted-foreground"
                >
                  No backups yet.
                </td>
              </tr>
            ) : (
              backups.map((b) => (
                <tr
                  key={b.filename}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="px-4 py-3 font-mono text-xs max-w-xs truncate">
                    {b.filename}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {formatSize(b.size_bytes)}
                  </td>
                  <td
                    className="px-4 py-3 text-muted-foreground"
                    title={new Date(b.created_at).toLocaleString()}
                  >
                    {timeAgo(b.created_at)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          size="icon-sm"
                          variant="ghost"
                          disabled={downloadingFile === b.filename}
                          aria-label={`Actions for ${b.filename}`}
                        >
                          <MoreVertical className="size-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent>
                        <DropdownMenuItem
                          onClick={() => handleDownload(b)}
                          disabled={downloadingFile === b.filename}
                        >
                          <Download className="size-3.5" />
                          {downloadingFile === b.filename
                            ? "Downloading…"
                            : "Download"}
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setDeleteTarget(b)}
                        >
                          <Trash2 className="size-3.5" /> Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Delete confirm dialog */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete this backup?</DialogTitle>
            <DialogDescription>
              {deleteTarget
                ? `This permanently removes "${deleteTarget.filename}". This can't be undone.`
                : "This can't be undone."}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter showCloseButton>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteSubmitting}
            >
              {deleteSubmitting ? "Deleting…" : "Delete backup"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Restore instructions dialog */}
      <Dialog open={restoreDialogOpen} onOpenChange={setRestoreDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Restoring from a backup</DialogTitle>
            <DialogDescription>
              There&apos;s no in-app restore button on purpose — swapping a live
              SQLite database out from under a running server safely isn&apos;t
              worth the risk for a single-admin app. Stop the container first,
              then do this on the host:
            </DialogDescription>
          </DialogHeader>
          <ol className="list-decimal pl-5 flex flex-col gap-2 text-sm">
            <li>
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                docker compose stop bookshelf-backend
              </code>{" "}
              (the frontend can stay up; it&apos;ll show fetch errors until the
              backend restarts).
            </li>
            <li>
              Extract the downloaded archive on the host:{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                tar -xzf bookshelf-backup-&lt;ts&gt;.tar.gz -C /tmp/restore
              </code>
              .
            </li>
            <li>
              Copy the volume&apos;s current contents aside as a safety net —
              it&apos;s a named volume, not a bind mount:{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded break-all">
                docker run --rm -v bookshelf-data:/data -v
                $(pwd)/pre-restore-backup:/backup alpine cp -r /data/. /backup/
              </code>
              .
            </li>
            <li>
              Replace{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                bookshelf.db
              </code>{" "}
              and{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                covers/
              </code>{" "}
              inside the volume with the extracted ones, same{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                docker run -v bookshelf-data:/data ...
              </code>{" "}
              pattern as step 3.
            </li>
            <li>
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                docker compose start bookshelf-backend
              </code>
              .
            </li>
            <li>
              Verify via{" "}
              <code className="text-xs bg-muted px-1 py-0.5 rounded">
                GET /health
              </code>
              , then spot-check book/user counts against expectations in this
              admin dashboard.
            </li>
          </ol>
          <p className="text-xs text-muted-foreground">
            Full details in the backend&apos;s CLAUDE.md (&quot;Backup and
            restore&quot;).
          </p>
          <DialogFooter showCloseButton />
        </DialogContent>
      </Dialog>
    </div>
  );
}
