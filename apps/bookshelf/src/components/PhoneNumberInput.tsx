"use client";

import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { COUNTRY_CODES, COUNTRY_CODES_BY_ISO2 } from "@/lib/countryCodes";
import { parsePhone } from "@/lib/parsePhone";
import { cn } from "@/lib/utils";

interface PhoneNumberInputProps {
  id: string;
  /** Full phone string, e.g. "+65 9123 4567", or "" when empty. */
  value: string;
  /** Called with the recomposed full "+<dialcode> <local>" string (or "" if local is blank). */
  onChange: (value: string) => void;
  placeholder?: string;
  autoComplete?: string;
  className?: string;
}

export function PhoneNumberInput({
  id,
  value,
  onChange,
  placeholder,
  autoComplete,
  className,
}: PhoneNumberInputProps) {
  // Re-derived on every render from `value` — the parent owns the single
  // source of truth, and every keystroke below calls onChange synchronously,
  // so there's no local-only state here that needs preserving separately.
  const { iso2, localNumber } = parsePhone(value);

  function compose(nextIso2: string, nextLocal: string): string {
    const trimmedLocal = nextLocal.trim();
    if (!trimmedLocal) return "";
    const dialCode =
      COUNTRY_CODES_BY_ISO2[nextIso2]?.dialCode ??
      COUNTRY_CODES_BY_ISO2["SG"].dialCode;
    return `+${dialCode} ${trimmedLocal}`;
  }

  return (
    <div className={cn("flex gap-2", className)}>
      <Select
        value={iso2}
        onValueChange={(nextIso2) => onChange(compose(nextIso2, localNumber))}
      >
        <SelectTrigger className="w-[100px] shrink-0" aria-label="Country code">
          <SelectValue>+{COUNTRY_CODES_BY_ISO2[iso2]?.dialCode}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {COUNTRY_CODES.map((c) => (
            <SelectItem key={c.iso2} value={c.iso2}>
              {c.name} (+{c.dialCode})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        id={id}
        type="tel"
        autoComplete={autoComplete}
        value={localNumber}
        onChange={(e) => onChange(compose(iso2, e.target.value))}
        placeholder={placeholder}
        className="flex-1"
      />
    </div>
  );
}
