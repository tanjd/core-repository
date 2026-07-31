// Shared formatting utilities for numbers, currencies, percentages, and dates.

const usdFormatter = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const sgdFormatter = new Intl.NumberFormat("en-SG", {
  style: "currency",
  currency: "SGD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const compactFormatter = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  notation: "compact",
  maximumFractionDigits: 1,
});

export function fmtUsd(value: number): string {
  return usdFormatter.format(value);
}

export function fmtSgd(value: number): string {
  return sgdFormatter.format(value);
}

export function fmtCurrency(value: number, currency: string): string {
  if (currency === "SGD") return fmtSgd(value);
  return fmtUsd(value);
}

export function fmtCompact(value: number): string {
  return compactFormatter.format(value);
}

export function fmtPct(value: number, decimals = 2): string {
  return `${value >= 0 ? "+" : ""}${value.toFixed(decimals)}%`;
}

export function fmtPctPlain(value: number, decimals = 2): string {
  return `${value.toFixed(decimals)}%`;
}

export function fmtQty(value: number): string {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  }).format(value);
}

/** Returns Tailwind color class for positive/negative/zero values. */
export function pnlColor(value: number): string {
  if (value > 0) return "text-green-600 dark:text-green-400";
  if (value < 0) return "text-red-500 dark:text-red-400";
  return "text-muted-foreground";
}

/** Format ISO date string as "DD MMM YYYY". */
export function fmtDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

/** Format "2025-01" month key as "Jan 2025". */
export function fmtMonth(monthKey: string): string {
  const [year, month] = monthKey.split("-");
  const d = new Date(Number(year), Number(month) - 1, 1);
  return d.toLocaleDateString("en-GB", { month: "short", year: "numeric" });
}
