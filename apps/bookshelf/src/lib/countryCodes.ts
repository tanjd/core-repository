export interface CountryCode {
  /** ISO 3166-1 alpha-2, e.g. "SG". Used as the Select item's value/key. */
  iso2: string;
  /** Display name, e.g. "Singapore". */
  name: string;
  /** Dial code WITHOUT the leading "+", e.g. "65", "1", "44". */
  dialCode: string;
}

// Curated, not exhaustive ISO-3166 — big enough that no plausible member is
// stuck, small enough to stay a literal array a human can review/extend.
// Singapore is first (this app's original community, and the parsePhone
// fallback default); the rest are alphabetical by name.
export const COUNTRY_CODES: CountryCode[] = [
  { iso2: "SG", name: "Singapore", dialCode: "65" },
  { iso2: "AE", name: "United Arab Emirates", dialCode: "971" },
  { iso2: "AU", name: "Australia", dialCode: "61" },
  { iso2: "BR", name: "Brazil", dialCode: "55" },
  { iso2: "CA", name: "Canada", dialCode: "1" },
  { iso2: "CH", name: "Switzerland", dialCode: "41" },
  { iso2: "CN", name: "China", dialCode: "86" },
  { iso2: "DE", name: "Germany", dialCode: "49" },
  { iso2: "ES", name: "Spain", dialCode: "34" },
  { iso2: "FR", name: "France", dialCode: "33" },
  { iso2: "GB", name: "United Kingdom", dialCode: "44" },
  { iso2: "HK", name: "Hong Kong", dialCode: "852" },
  { iso2: "ID", name: "Indonesia", dialCode: "62" },
  { iso2: "IE", name: "Ireland", dialCode: "353" },
  { iso2: "IN", name: "India", dialCode: "91" },
  { iso2: "IT", name: "Italy", dialCode: "39" },
  { iso2: "JP", name: "Japan", dialCode: "81" },
  { iso2: "KR", name: "South Korea", dialCode: "82" },
  { iso2: "MX", name: "Mexico", dialCode: "52" },
  { iso2: "MY", name: "Malaysia", dialCode: "60" },
  { iso2: "NL", name: "Netherlands", dialCode: "31" },
  { iso2: "NZ", name: "New Zealand", dialCode: "64" },
  { iso2: "PH", name: "Philippines", dialCode: "63" },
  { iso2: "SA", name: "Saudi Arabia", dialCode: "966" },
  { iso2: "SE", name: "Sweden", dialCode: "46" },
  { iso2: "TH", name: "Thailand", dialCode: "66" },
  { iso2: "TW", name: "Taiwan", dialCode: "886" },
  { iso2: "US", name: "United States", dialCode: "1" },
  { iso2: "VN", name: "Vietnam", dialCode: "84" },
  { iso2: "ZA", name: "South Africa", dialCode: "27" },
];

export const DEFAULT_COUNTRY_ISO2 = "SG";

// iso2 -> CountryCode, built once at module load for O(1) lookups.
export const COUNTRY_CODES_BY_ISO2: Record<string, CountryCode> =
  Object.fromEntries(COUNTRY_CODES.map((c) => [c.iso2, c]));
