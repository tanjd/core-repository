import { Check, X } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Live match feedback for a confirm-password field, mirroring the "don't
 * show red until the user has engaged" contract of PasswordStrengthMeter.
 * Deliberately never shows red for a mismatch — the confirm field is
 * expected not to match on every keystroke but the last, so a muted state
 * (not an error color) avoids training users to ignore it.
 */
export function PasswordMatchIndicator({
  password,
  confirm,
}: {
  password: string;
  confirm: string;
}) {
  if (!confirm) return null;

  const matches = password === confirm;
  return (
    <p
      className={cn(
        "flex items-center gap-1.5 text-xs",
        matches ? "text-success" : "text-muted-foreground",
      )}
    >
      {matches ? (
        <Check className="size-3.5 shrink-0" aria-hidden />
      ) : (
        <X className="size-3.5 shrink-0" aria-hidden />
      )}
      {matches ? "Matches" : "Doesn't match yet"}
    </p>
  );
}
