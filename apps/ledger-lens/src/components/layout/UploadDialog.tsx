"use client";

import { useCallback, useRef, useState } from "react";
import {
  Upload,
  AlertCircle,
  CheckCircle2,
  RefreshCw,
  Loader2,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { previewStatement, uploadStatement } from "@/lib/api";
import type { PreviewResponse, UploadResponse } from "@/lib/types";
import { fmtUsd, fmtPct } from "@/lib/formatters";
import { revalidateAll } from "@/hooks/useStatement";
import { useYear } from "@/context/YearContext";

type FileStatus = "previewing" | "ready" | "importing" | "done" | "error";

interface FileEntry {
  file: File;
  status: FileStatus;
  preview?: PreviewResponse;
  result?: UploadResponse;
  error?: string;
}

interface EntryRowProps {
  entry: FileEntry;
}

const BROKER_LABELS: Record<string, string> = {
  ibkr: "IBKR",
  moomoo: "Moomoo",
};

function PreviewDetails({ preview }: { preview: PreviewResponse }) {
  const brokerLabel =
    BROKER_LABELS[preview.broker] ?? preview.broker.toUpperCase();
  const isMoomooTrades =
    preview.broker === "moomoo" && (preview.years_detected?.length ?? 0) > 0;
  const isMoomooPositions = preview.broker === "moomoo" && !isMoomooTrades;

  if (isMoomooTrades) {
    const years = preview.years_detected!;
    const yearRange =
      years.length === 1
        ? `${years[0]}`
        : `${years[0]}–${years[years.length - 1]}`;
    return (
      <div className="pl-6 space-y-0.5 text-xs text-muted-foreground">
        <span>
          {brokerLabel} · Trade history · {yearRange} · {preview.trade_count}{" "}
          trades
        </span>
        {preview.already_imported && (
          <span className="flex items-center gap-1 text-yellow-600 dark:text-yellow-400">
            <RefreshCw className="h-3 w-3 shrink-0" />
            Some years already imported — will replace
          </span>
        )}
      </div>
    );
  }

  if (isMoomooPositions) {
    return (
      <div className="pl-6 space-y-0.5 text-xs text-muted-foreground">
        {preview.already_imported && (
          <span className="flex items-center gap-1 text-yellow-600 dark:text-yellow-400">
            <RefreshCw className="h-3 w-3 shrink-0" />
            {preview.year} already imported — will replace
          </span>
        )}
        <span>
          {brokerLabel} · Positions · {preview.year} ·{" "}
          {preview.period_end_label} · {fmtUsd(preview.nav_current)} ·{" "}
          {preview.position_count} positions
        </span>
      </div>
    );
  }

  // IBKR
  return (
    <div className="pl-6 space-y-0.5 text-xs text-muted-foreground">
      {preview.already_imported && (
        <span className="flex items-center gap-1 text-yellow-600 dark:text-yellow-400">
          <RefreshCw className="h-3 w-3 shrink-0" />
          {preview.year} already imported — will replace
        </span>
      )}
      <span>
        {brokerLabel} · {preview.year} · {preview.period_end_label} ·{" "}
        {fmtUsd(preview.nav_current)} · {fmtPct(preview.twr_pct)} TWR
      </span>
    </div>
  );
}

function ResultDetails({ result }: { result: UploadResponse }) {
  const brokerLabel =
    BROKER_LABELS[result.broker] ?? result.broker.toUpperCase();
  const parts: string[] = [];
  if (result.trade_count > 0) parts.push(`${result.trade_count} trades`);
  if (result.position_count > 0)
    parts.push(`${result.position_count} positions`);
  if (result.deposit_count > 0) parts.push(`${result.deposit_count} deposits`);
  if (result.dividend_count > 0)
    parts.push(`${result.dividend_count} dividends`);
  const countsLine = parts.join(" · ") || result.period_end_label;

  return (
    <div className="pl-6 space-y-0.5 text-xs text-muted-foreground">
      <span>
        {brokerLabel} · {result.year} · {result.period_end_label}
        {result.nav_current > 0 && ` · ${fmtUsd(result.nav_current)}`}
        {result.twr_pct !== 0 && ` · ${fmtPct(result.twr_pct)} TWR`}
      </span>
      {countsLine && <span className="block">{countsLine}</span>}
    </div>
  );
}

function EntryRow({ entry }: EntryRowProps) {
  const { file, status, preview, result, error } = entry;
  return (
    <div className="rounded-md border p-3 text-sm space-y-1">
      <div className="flex items-center gap-2">
        {(status === "previewing" || status === "importing") && (
          <Loader2 className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
        )}
        {status === "ready" && (
          <CheckCircle2 className="h-4 w-4 shrink-0 text-muted-foreground" />
        )}
        {status === "done" && (
          <CheckCircle2 className="h-4 w-4 shrink-0 text-green-600 dark:text-green-400" />
        )}
        {status === "error" && (
          <AlertCircle className="h-4 w-4 shrink-0 text-destructive" />
        )}
        <span className="font-medium truncate">{file.name}</span>
      </div>

      {error && <p className="pl-6 text-xs text-destructive">{error}</p>}

      {status === "done" && result ? (
        <ResultDetails result={result} />
      ) : (
        preview && status !== "error" && <PreviewDetails preview={preview} />
      )}
    </div>
  );
}

export function UploadDialog() {
  const [open, setOpen] = useState(false);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [importing, setImporting] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const { refreshYears } = useYear();

  const updateEntry = useCallback(
    (index: number, patch: Partial<FileEntry>) => {
      setEntries((prev) =>
        prev.map((e, i) => (i === index ? { ...e, ...patch } : e)),
      );
    },
    [],
  );

  const reset = useCallback(() => {
    setEntries([]);
    setImporting(false);
    if (inputRef.current) inputRef.current.value = "";
  }, []);

  const handleOpenChange = useCallback(
    (val: boolean) => {
      setOpen(val);
      if (!val) reset();
    },
    [reset],
  );

  const handleFileChange = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(e.target.files ?? []);
      if (files.length === 0) return;

      setEntries(files.map((file) => ({ file, status: "previewing" })));

      await Promise.all(
        files.map(async (file, i) => {
          try {
            const preview = await previewStatement(file);
            updateEntry(i, { status: "ready", preview });
          } catch (err) {
            updateEntry(i, {
              status: "error",
              error: err instanceof Error ? err.message : "Preview failed",
            });
          }
        }),
      );
    },
    [updateEntry],
  );

  const handleImport = useCallback(async () => {
    setImporting(true);
    let imported = 0;

    for (let i = 0; i < entries.length; i++) {
      if (entries[i].status !== "ready") continue;
      updateEntry(i, { status: "importing" });
      try {
        const result = await uploadStatement(entries[i].file);
        updateEntry(i, { status: "done", result });
        const parts: string[] = [];
        if (result.trade_count > 0) parts.push(`${result.trade_count} trades`);
        if (result.position_count > 0)
          parts.push(`${result.position_count} positions`);
        if (result.deposit_count > 0)
          parts.push(`${result.deposit_count} deposits`);
        toast.success(`Imported ${result.year} · ${result.account_id}`, {
          description:
            parts.length > 0 ? parts.join(" · ") : result.period_end_label,
        });
        imported++;
      } catch (err) {
        updateEntry(i, {
          status: "error",
          error: err instanceof Error ? err.message : "Import failed",
        });
      }
    }

    if (imported > 0) {
      revalidateAll();
      refreshYears();
    }

    setImporting(false);
  }, [entries, updateEntry, refreshYears]);

  const readyCount = entries.filter((e) => e.status === "ready").length;
  const allSettled =
    entries.length > 0 &&
    entries.every((e) => e.status === "done" || e.status === "error");

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" className="gap-1.5">
          <Upload className="h-3.5 w-3.5" />
          Import CSV
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Import Statements</DialogTitle>
          <DialogDescription>
            Upload IBKR activity statement CSVs
            (e.g.&nbsp;U11111111_2025_2025.csv) or Moomoo exports — trade
            history (History-Margin Account…) and positions (Positions-Margin
            Account…).
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          {/* File picker */}
          <label
            className={`flex cursor-pointer flex-col items-center gap-2 rounded-lg border-2 border-dashed p-4 transition-colors hover:border-primary ${importing ? "pointer-events-none opacity-50" : ""}`}
          >
            <Upload className="h-6 w-6 text-muted-foreground" />
            <span className="text-sm font-medium">
              Click to select CSV files
            </span>
            <span className="text-xs text-muted-foreground">
              IBKR · Moomoo trade history · Moomoo positions
            </span>
            <input
              ref={inputRef}
              type="file"
              accept=".csv"
              multiple
              className="sr-only"
              onChange={handleFileChange}
              disabled={importing}
            />
          </label>

          {/* File list */}
          {entries.length > 0 && (
            <div className="max-h-64 space-y-2 overflow-y-auto">
              {entries.map((entry, i) => (
                <EntryRow key={i} entry={entry} />
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="ghost"
            onClick={() => setOpen(false)}
            disabled={importing}
          >
            {allSettled ? "Close" : "Cancel"}
          </Button>
          {readyCount > 0 && !allSettled && (
            <Button onClick={handleImport} disabled={importing}>
              {importing
                ? "Importing…"
                : `Import ${readyCount} file${readyCount !== 1 ? "s" : ""}`}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
