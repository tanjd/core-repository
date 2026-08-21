"use client";

import { useState } from "react";
import Image from "next/image";
import { cn } from "@/lib/utils";
import { gradientForTitle, hashString, wrapLines } from "@/lib/bookCoverColors";

const VIEWBOX_WIDTH = 300;
const VIEWBOX_HEIGHT = 450;

interface TextLine {
  text: string;
  y: number;
  fontSize: number;
  fontWeight: number;
}

// Stacks title lines (bold) above an optional single author line, anchored
// to the bottom of the cover with fixed padding, so the block's vertical
// position adapts to however many title lines got wrapped.
function buildLines(titleLines: string[], authorLines: string[]): TextLine[] {
  const titleLineHeight = 34;
  const authorLineHeight = 24;
  const gap = 10;
  const bottomPadding = 30;

  const rows: {
    text: string;
    height: number;
    fontSize: number;
    fontWeight: number;
    render: boolean;
  }[] = titleLines.map((text) => ({
    text,
    height: titleLineHeight,
    fontSize: 28,
    fontWeight: 700,
    render: true,
  }));

  if (authorLines.length > 0) {
    rows.push({
      text: "",
      height: gap,
      fontSize: 0,
      fontWeight: 0,
      render: false,
    });
    for (const text of authorLines) {
      rows.push({
        text,
        height: authorLineHeight,
        fontSize: 18,
        fontWeight: 500,
        render: true,
      });
    }
  }

  const totalHeight = rows.reduce((sum, row) => sum + row.height, 0);
  let cursor = VIEWBOX_HEIGHT - bottomPadding - totalHeight;
  const lines: TextLine[] = [];

  for (const row of rows) {
    cursor += row.height;
    if (row.render) {
      lines.push({
        text: row.text,
        y: cursor - row.height / 2,
        fontSize: row.fontSize,
        fontWeight: row.fontWeight,
      });
    }
  }

  return lines;
}

interface BookCoverFallbackProps {
  title: string;
  author?: string | null;
  className?: string;
}

// A deterministic, presentable "generated cover" — a soft gradient derived
// from the title (so the same book always looks the same) with the title
// and author centered over it, in the spirit of Spotify's generated
// playlist covers. Scales purely via the SVG viewBox, so one component
// serves every size this app renders a cover at (spine strip, thumbnail,
// detail page) with no per-site sizing prop.
export function BookCoverFallback({
  title,
  author,
  className,
}: BookCoverFallbackProps) {
  const { from, to } = gradientForTitle(title);
  const gradientId = `book-cover-gradient-${hashString(title)}`;
  const fillId = `${gradientId}-fill`;
  const scrimId = `${gradientId}-scrim`;

  const titleLines = wrapLines(title, 15, 3);
  const authorLines = author ? wrapLines(author, 24, 1) : [];
  const lines = buildLines(titleLines, authorLines);

  const label = author
    ? `Cover placeholder for ${title}, by ${author}`
    : `Cover placeholder for ${title}`;

  return (
    <svg
      viewBox={`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`}
      preserveAspectRatio="xMidYMid slice"
      role="img"
      aria-label={label}
      className={cn("block h-full w-full", className)}
    >
      <defs>
        <linearGradient id={fillId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor={from} />
          <stop offset="100%" stopColor={to} />
        </linearGradient>
        <linearGradient id={scrimId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="55%" stopColor="black" stopOpacity="0" />
          <stop offset="100%" stopColor="black" stopOpacity="0.45" />
        </linearGradient>
      </defs>
      <rect
        width={VIEWBOX_WIDTH}
        height={VIEWBOX_HEIGHT}
        fill={`url(#${fillId})`}
      />
      <rect
        width={VIEWBOX_WIDTH}
        height={VIEWBOX_HEIGHT}
        fill={`url(#${scrimId})`}
      />
      {lines.map((line, index) => (
        <text
          key={index}
          x={VIEWBOX_WIDTH / 2}
          y={line.y}
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize={line.fontSize}
          fontWeight={line.fontWeight}
          fill="white"
          fontFamily="system-ui, sans-serif"
        >
          {line.text}
        </text>
      ))}
    </svg>
  );
}

interface BookCoverProps {
  title: string;
  author?: string | null;
  coverUrl?: string | null;
  alt?: string;
  sizes: string;
  className?: string;
  priority?: boolean;
}

// Drop-in replacement for the `cover_url ? <Image/> : <fallback/>` ternary
// every book-rendering surface used to hand-roll — renders the real cover
// photo when there is one, otherwise a generated BookCoverFallback. Also
// falls back to BookCoverFallback if a present coverUrl fails to load (404,
// dead host, CORS), rather than leaving a broken image in place. Client
// Component (needs the load-error state), so every call site must already
// be within a Client Component boundary.
export function BookCover({
  title,
  author,
  coverUrl,
  alt,
  sizes,
  className,
  priority,
}: BookCoverProps) {
  const [failed, setFailed] = useState(false);

  if (coverUrl && !failed) {
    return (
      <Image
        src={coverUrl}
        alt={alt ?? `Cover of ${title}`}
        fill
        sizes={sizes}
        priority={priority}
        className={cn("object-cover", className)}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <BookCoverFallback title={title} author={author} className={className} />
  );
}
