"use client";

import { useState } from "react";
import { Pencil } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { LoanRequest } from "@/lib/types";
import { timeAgo } from "@/lib/timeFormat";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

// expected_return_date comes back as a full RFC3339 timestamp; <input
// type="date"> needs a bare YYYY-MM-DD.
function toDateInputValue(iso?: string): string {
  return iso ? iso.slice(0, 10) : "";
}

// The viewer's own id isn't passed down as a prop (this cell is used
// read-only by both the borrower's and the owner's requests pages) — read it
// straight from the same localStorage key every page's auth guard already
// relies on, so we can tell which side of the loan the current viewer is on
// without threading a currentUser prop through two call sites.
function currentUserId(): number | null {
  try {
    const stored = localStorage.getItem("bookshelf_user");
    if (!stored) return null;
    const user = JSON.parse(stored) as { id?: number };
    return typeof user.id === "number" ? user.id : null;
  } catch {
    return null;
  }
}

// Shows the agreed return date for an accepted loan, with an inline
// affordance to set/change it — the date is otherwise locked in at request
// time and often left blank, so either party can fill it in later once
// something's actually been agreed (e.g. over chat).
export function ReturnDateCell({
  request,
  onUpdated,
}: {
  request: LoanRequest;
  onUpdated: (updated: LoanRequest) => void;
}) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState(
    toDateInputValue(request.expected_return_date),
  );
  const [saving, setSaving] = useState(false);

  async function handleSave() {
    if (!value) return;
    setSaving(true);
    try {
      const updated = await api.updateExpectedReturnDate(request.id, value);
      onUpdated(updated);
      setOpen(false);
      const viewerIsBorrower = currentUserId() === request.borrower_id;
      const otherPartyName = viewerIsBorrower
        ? (request.copy?.owner?.name ?? "the owner")
        : (request.borrower?.name ?? "the borrower");
      toast.success(
        `Return date updated — ${otherPartyName} has been notified`,
      );
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to update return date",
      );
    } finally {
      setSaving(false);
    }
  }

  const isOverdue =
    request.status === "accepted" &&
    !!request.expected_return_date &&
    new Date(request.expected_return_date) < new Date();

  const amendedByName = !request.expected_return_date_changed_by
    ? null
    : request.expected_return_date_changed_by === request.borrower_id
      ? (request.borrower?.name ?? "the borrower")
      : (request.copy?.owner?.name ?? "the owner");

  return (
    <div className="flex flex-col gap-0.5">
      <div className="flex items-center gap-1">
        <span className={cn(isOverdue && "text-destructive font-medium")}>
          {request.expected_return_date
            ? new Date(request.expected_return_date).toLocaleDateString()
            : "No return date agreed"}
        </span>
        {isOverdue && <Badge variant="destructive">Overdue</Badge>}
        {request.status === "accepted" && (
          <Popover
            open={open}
            onOpenChange={(next) => {
              setValue(toDateInputValue(request.expected_return_date));
              setOpen(next);
            }}
          >
            <PopoverTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-6"
                onClick={(e) => e.stopPropagation()}
                aria-label="Edit return date"
              >
                <Pencil className="size-3" />
              </Button>
            </PopoverTrigger>
            <PopoverContent
              className="w-auto flex flex-col gap-2"
              onClick={(e) => e.stopPropagation()}
            >
              <label className="text-xs font-medium text-muted-foreground">
                Return by
              </label>
              <Input
                id="edit-return-date"
                type="date"
                min={toDateInputValue(new Date().toISOString())}
                value={value}
                onChange={(e) => setValue(e.target.value)}
              />
              <Button
                size="sm"
                onClick={handleSave}
                disabled={saving || !value}
              >
                {saving ? "Saving…" : "Save"}
              </Button>
            </PopoverContent>
          </Popover>
        )}
      </div>
      {amendedByName && request.expected_return_date_changed_at && (
        <span className="text-xs text-muted-foreground">
          Amended by {amendedByName}{" "}
          {timeAgo(request.expected_return_date_changed_at)}
        </span>
      )}
    </div>
  );
}
