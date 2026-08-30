"use client";

import { cn } from "@/lib/utils";

interface SegmentedControlOption<T extends string> {
  value: T;
  label: string;
}

interface SegmentedControlProps<T extends string> {
  value: T;
  onValueChange: (value: T) => void;
  options: SegmentedControlOption<T>[];
  "aria-label": string;
  className?: string;
}

// A lightweight, visually quiet toggle for filtering a list in place —
// deliberately not built on the Tabs primitive, which reads as a primary
// navigational choice (see the outer Borrowing/Lending tabs on /loans). This
// is for the secondary "which subset of this list" filter that sits inside
// a tab, so it should look smaller and lower-contrast than the tabs
// containing it, not compete with them.
export function SegmentedControl<T extends string>({
  value,
  onValueChange,
  options,
  className,
  ...rest
}: SegmentedControlProps<T>) {
  return (
    <div
      role="group"
      aria-label={rest["aria-label"]}
      className={cn(
        "inline-flex items-center gap-0.5 rounded-full bg-muted p-0.5",
        className,
      )}
    >
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={active}
            onClick={() => onValueChange(option.value)}
            className={cn(
              "rounded-full px-3 py-1 text-sm font-medium transition-colors",
              active
                ? "bg-background text-foreground shadow-xs"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
