import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { InitialsAvatar } from "@/components/InitialsAvatar";
import type { Copy } from "@/lib/types";
import { cn } from "@/lib/utils";

interface CopyCardProps {
  copy: Copy;
  actions?: ReactNode;
  // highlighted flags this copy as the "best pick" — used by the book
  // detail hero to visually tie the top CTA to the specific copy it
  // acts on, so users can see which one they're about to borrow.
  highlighted?: boolean;
  // demoted collapses styling for copies the user can't act on right
  // now (loaned/unavailable) so the eye can skip them.
  demoted?: boolean;
}

const conditionLabel: Record<Copy["condition"], string> = {
  good: "Good",
  fair: "Fair",
  worn: "Worn",
};

const statusLabel: Record<Copy["status"], string> = {
  available: "Available",
  requested: "Requested",
  loaned: "On Loan",
  unavailable: "Unavailable",
};

// Status is shown as a leading colored dot + text, not another Badge —
// the previous design had 2–4 badges of equal weight per row, which made
// the actually-important signal (status) fight condition and
// instant-approval for attention. Colors map to the app-wide
// success/destructive/secondary/outline vocabulary via CSS variables.
const statusDotClass: Record<Copy["status"], string> = {
  available: "bg-success",
  requested: "bg-secondary-foreground/50",
  loaned: "bg-destructive",
  unavailable: "bg-muted-foreground/50",
};

export function CopyCard({
  copy,
  actions,
  highlighted,
  demoted,
}: CopyCardProps) {
  const ownerName = copy.hide_owner
    ? "Anonymous member"
    : (copy.owner?.name ?? null);

  return (
    <Card
      className={cn(
        // h-full lets the card stretch to the tallest sibling in a
        // CSS-grid row — otherwise a "Best pick" card (with a short
        // text hint) sits shorter than a sibling that renders a full
        // Button, and the two look ragged side-by-side on desktop.
        "py-4 h-full transition-colors",
        highlighted && "border-primary/60 bg-primary/5",
        demoted && "opacity-75",
      )}
      data-status={copy.status}
      data-highlighted={highlighted ? "true" : undefined}
    >
      <CardContent className="px-4 flex flex-col gap-2 h-full">
        {/* Line 1: owner + status. Owner is the trust signal; status is
            the one facet the user needs to decide if they can act. */}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            {ownerName ? (
              <InitialsAvatar name={ownerName} />
            ) : (
              <InitialsAvatar name="?" />
            )}
            <div className="flex flex-col min-w-0">
              {ownerName ? (
                <span className="text-sm font-medium truncate">
                  {ownerName}
                </span>
              ) : (
                <span className="text-xs text-muted-foreground">
                  Sign in and verify to see who&apos;s sharing
                </span>
              )}
              <span
                className="flex items-center gap-1.5 text-xs text-muted-foreground"
                aria-label={`Status: ${statusLabel[copy.status]}`}
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    "inline-block size-2 rounded-full",
                    statusDotClass[copy.status],
                  )}
                />
                {statusLabel[copy.status]}
                {highlighted && copy.status === "available" && (
                  <span className="text-primary font-medium">· Best pick</span>
                )}
                {copy.status === "available" && copy.auto_approve && (
                  <span className="text-primary">· Instant approval</span>
                )}
              </span>
            </div>
          </div>
        </div>

        {/* Line 2: condition + notes. Condition is a small detail, not a
            headline — a subtle badge is enough. */}
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <Badge variant="outline" className="text-xs font-normal">
            {conditionLabel[copy.condition]} condition
          </Badge>
          {copy.notes && (
            <p className="text-sm text-muted-foreground italic">{copy.notes}</p>
          )}
        </div>

        {copy.status === "requested" && (
          <p className="text-xs text-muted-foreground">
            Someone&apos;s already asked to borrow this — join the waitlist to
            be notified if it opens back up.
          </p>
        )}

        {actions && (
          // mt-auto pushes the action row to the bottom of the flex
          // column so cards with different action heights (a button
          // vs. a short text hint) still line up cleanly at the base.
          <div className="flex flex-wrap items-center gap-2 mt-auto pt-1">
            {actions}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
