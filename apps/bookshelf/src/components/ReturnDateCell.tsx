"use client";

import { useState } from "react";
import { Pencil } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { LoanRequest } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

// expected_return_date comes back as a full RFC3339 timestamp; <input
// type="date"> needs a bare YYYY-MM-DD.
function toDateInputValue(iso?: string): string {
  return iso ? iso.slice(0, 10) : "";
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
      toast.success("Return date updated");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to update return date",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex items-center gap-1">
      <span>
        {request.expected_return_date
          ? new Date(request.expected_return_date).toLocaleDateString()
          : "No return date agreed"}
      </span>
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
              type="date"
              value={value}
              onChange={(e) => setValue(e.target.value)}
            />
            <Button size="sm" onClick={handleSave} disabled={saving || !value}>
              {saving ? "Saving…" : "Save"}
            </Button>
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
