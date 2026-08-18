"use client";

import { Check, X } from "lucide-react";
import {
  MIN_PASSWORD_LENGTH,
  scorePasswordStrength,
  type PasswordStrengthScore,
} from "@/lib/api";
import { cn } from "@/lib/utils";

const BAR_COLORS: Record<PasswordStrengthScore, string> = {
  0: "bg-destructive",
  1: "bg-destructive",
  2: "bg-amber-500",
  3: "bg-amber-500",
  4: "bg-success",
};

const LABEL_COLORS: Record<PasswordStrengthScore, string> = {
  0: "text-destructive",
  1: "text-destructive",
  2: "text-amber-600 dark:text-amber-400",
  3: "text-amber-600 dark:text-amber-400",
  4: "text-success",
};

interface Requirement {
  label: string;
  met: boolean;
}

function requirementsFor(
  password: string,
  disallowed: string[],
): Requirement[] {
  const lower = password.toLowerCase();
  const disallowedHit = disallowed.some((d) => {
    const needle = d.trim().toLowerCase();
    return needle.length >= 3 && lower.includes(needle);
  });

  return [
    {
      label: `At least ${MIN_PASSWORD_LENGTH} characters`,
      met: password.length >= MIN_PASSWORD_LENGTH,
    },
    {
      label: "Uppercase and lowercase letters",
      met: /[A-Z]/.test(password) && /[a-z]/.test(password),
    },
    { label: "At least one number", met: /[0-9]/.test(password) },
    { label: "Doesn't contain your name or email", met: !disallowedHit },
  ];
}

/**
 * Live password-strength feedback for signup/change-password forms: a
 * 4-segment strength bar plus a checklist of the concrete requirements
 * validatePassword enforces server-side. Renders nothing until the user has
 * typed something, so empty fields don't show a wall of red X's.
 */
export function PasswordStrengthMeter({
  password,
  disallowed = [],
}: {
  password: string;
  disallowed?: string[];
}) {
  if (!password) return null;

  const { score, label } = scorePasswordStrength(password);
  const requirements = requirementsFor(password, disallowed);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <div className="flex flex-1 gap-1" role="presentation">
          {[0, 1, 2, 3].map((segment) => (
            <div
              key={segment}
              className={cn(
                "h-1.5 flex-1 rounded-full bg-muted transition-colors",
                segment < score && BAR_COLORS[score],
              )}
            />
          ))}
        </div>
        <span className={cn("text-xs font-medium", LABEL_COLORS[score])}>
          {label}
        </span>
      </div>
      <ul className="grid grid-cols-1 gap-1 sm:grid-cols-2">
        {requirements.map((req) => (
          <li
            key={req.label}
            className={cn(
              "flex items-center gap-1.5 text-xs",
              req.met ? "text-success" : "text-muted-foreground",
            )}
          >
            {req.met ? (
              <Check className="size-3.5 shrink-0" />
            ) : (
              <X className="size-3.5 shrink-0" />
            )}
            {req.label}
          </li>
        ))}
      </ul>
    </div>
  );
}
