import { AlertCircle } from "lucide-react";

interface Props {
  /** The error thrown by the fetcher (see lib/api.ts's `fetcher`). */
  error: Error;
  /** Use inside a smaller section instead of a full-page block. */
  compact?: boolean;
}

/** Shown instead of an empty state when a request actually failed, so a
 * backend outage doesn't read as "no data imported yet". */
export function ApiErrorState({ error, compact }: Props) {
  return (
    <div
      className={
        compact
          ? "flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4"
          : "flex h-full flex-col items-center justify-center gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-8 text-center"
      }
    >
      <AlertCircle className="h-5 w-5 shrink-0 text-destructive" />
      <div className={compact ? "space-y-0.5" : "space-y-1"}>
        <p className="text-sm font-medium text-destructive">
          Something went wrong loading data
        </p>
        <p className="text-xs text-muted-foreground">{error.message}</p>
      </div>
    </div>
  );
}
