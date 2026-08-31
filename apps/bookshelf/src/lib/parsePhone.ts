import { COUNTRY_CODES, DEFAULT_COUNTRY_ISO2 } from "@/lib/countryCodes";

export interface ParsedPhone {
  /** Best-guess country for the picker's selected value. */
  iso2: string;
  /** Remainder after the matched dial code, trimmed. */
  localNumber: string;
}

/**
 * Parses a stored "+<dialcode> <local>" (or no-space) string back into a
 * country + local number for the picker. Longest-dial-code-prefix match,
 * since multi-digit codes (852, 886, ...) must be tried before shorter ones
 * could wrongly match a substring.
 *
 * Falls back to DEFAULT_COUNTRY_ISO2 (Singapore, matching the previous
 * hardcoded behavior) with the full trimmed input as localNumber when phone
 * is empty/undefined or no known dial code matches.
 */
export function parsePhone(phone: string | null | undefined): ParsedPhone {
  const trimmed = (phone ?? "").trim();
  if (!trimmed.startsWith("+")) {
    return { iso2: DEFAULT_COUNTRY_ISO2, localNumber: trimmed };
  }
  const digits = trimmed.slice(1);
  const candidates = [...COUNTRY_CODES].sort(
    (a, b) => b.dialCode.length - a.dialCode.length,
  );
  const match = candidates.find((c) => digits.startsWith(c.dialCode));
  if (!match) {
    return { iso2: DEFAULT_COUNTRY_ISO2, localNumber: trimmed };
  }
  return {
    iso2: match.iso2,
    localNumber: digits.slice(match.dialCode.length).trim(),
  };
}
