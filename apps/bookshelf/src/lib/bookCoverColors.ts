// Deterministic 32-bit DJB2-xor hash — same string always produces the same
// number, no Math.random/Date, so it's stable across server and client
// renders (needed for generated book covers to not flicker/mismatch on
// hydration).
export function hashString(value: string): number {
  let hash = 5381;
  for (let i = 0; i < value.length; i++) {
    hash = (hash * 33) ^ value.charCodeAt(i);
  }
  return hash >>> 0;
}

export interface CoverGradient {
  from: string;
  to: string;
}

// Hashes the title (not author, which may be corrected later by metadata
// enrichment) into a hue, so the same book always gets the same generated
// cover. Fixed saturation/lightness bands keep every hue legible under white
// text without per-color contrast checks.
export function gradientForTitle(title: string): CoverGradient {
  const hue = hashString(title) % 360;
  const toHue = (hue + 42) % 360;
  return {
    from: `hsl(${hue} 55% 62%)`,
    to: `hsl(${toHue} 60% 42%)`,
  };
}

// Greedy word-wrap into at most maxLines lines of maxCharsPerLine, with a
// trailing ellipsis on the last line if content had to be cut off. No DOM
// measurement, so it works identically server- and client-side.
export function wrapLines(
  text: string,
  maxCharsPerLine: number,
  maxLines: number,
): string[] {
  const words = text.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0 || maxLines <= 0) return [];

  const lines: string[] = [];
  let current = "";
  let wordIndex = 0;

  while (wordIndex < words.length && lines.length < maxLines) {
    const word = words[wordIndex];
    const candidate = current ? `${current} ${word}` : word;

    if (candidate.length <= maxCharsPerLine || !current) {
      current = candidate;
      wordIndex++;
    } else {
      lines.push(current);
      current = "";
    }
  }

  if (current && lines.length < maxLines) {
    lines.push(current);
  }

  const isTruncated = wordIndex < words.length;
  if (isTruncated && lines.length > 0) {
    const lastIndex = lines.length - 1;
    let lastLine = lines[lastIndex];
    if (lastLine.length + 1 > maxCharsPerLine) {
      lastLine = lastLine.slice(0, Math.max(0, maxCharsPerLine - 1)).trimEnd();
    }
    lines[lastIndex] = `${lastLine}…`;
  }

  return lines;
}
