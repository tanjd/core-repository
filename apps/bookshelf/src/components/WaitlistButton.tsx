"use client";

import { useEffect, useState } from "react";
import { Clock, Users } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

interface WaitlistButtonProps {
  copyId: number;
}

export function WaitlistButton({ copyId }: WaitlistButtonProps) {
  const [count, setCount] = useState(0);
  const [onWaitlist, setOnWaitlist] = useState(false);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);

  useEffect(() => {
    api
      .getWaitlistStatus(copyId)
      .then((s) => {
        setCount(s.count);
        setOnWaitlist(s.on_waitlist);
      })
      .catch(() => {
        /* not authenticated or other error — ignore */
      })
      .finally(() => setLoading(false));
  }, [copyId]);

  async function toggle() {
    setActing(true);
    try {
      if (onWaitlist) {
        await api.leaveWaitlist(copyId);
        setCount((c) => Math.max(0, c - 1));
        setOnWaitlist(false);
        toast.success("Removed from waitlist");
      } else {
        await api.joinWaitlist(copyId);
        setCount((c) => c + 1);
        setOnWaitlist(true);
        toast.success(
          "Added to waitlist — you'll be notified when it's available",
        );
      }
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to update waitlist",
      );
    } finally {
      setActing(false);
    }
  }

  if (loading) return null;

  return (
    <div className="flex items-center gap-2">
      {count > 0 && (
        <Badge variant="secondary" className="gap-1 text-xs">
          <Users className="size-3" />
          {count} waiting
        </Badge>
      )}
      <Button
        size="sm"
        variant={onWaitlist ? "outline" : "secondary"}
        onClick={toggle}
        disabled={acting}
        className="gap-1.5"
      >
        <Clock className="size-3.5" />
        {onWaitlist ? "Leave Waitlist" : "Join Waitlist"}
      </Button>
    </div>
  );
}
