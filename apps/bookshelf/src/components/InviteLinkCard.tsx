"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// The exact detail string InviteCodeHandler returns when allow_invite_codes
// is off and the caller has no code yet — see
// internal/handlers/invite_codes.go's getInviteCode. Matched on to show the
// admin-disabled message rather than a generic failure.
const DISABLED_DETAIL = "invite links are currently disabled";

// "Invite a member" card for the profile page's Profile tab. Lazily creates
// the caller's invite link on first load (GET /invite-code is idempotent
// get-or-create) and offers a copy button plus a "Regenerate link" action
// that invalidates the old link. See apps/bookshelf/docs/invite-code-spec.md.
export function InviteLinkCard() {
  const [url, setUrl] = useState<string | null>(null);
  const [disabled, setDisabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [regenerating, setRegenerating] = useState(false);

  useEffect(() => {
    api
      .getInviteCode()
      .then(({ url: fetchedUrl }) => setUrl(fetchedUrl))
      .catch((err) => {
        if (err instanceof Error && err.message === DISABLED_DETAIL) {
          setDisabled(true);
        } else {
          toast.error(
            err instanceof Error
              ? err.message
              : "Could not load your invite link",
          );
        }
      })
      .finally(() => setLoading(false));
  }, []);

  async function copyLink() {
    if (!url) return;
    try {
      await navigator.clipboard.writeText(url);
      toast.success("Invite link copied");
    } catch {
      toast.error("Could not copy link");
    }
  }

  async function regenerate() {
    setRegenerating(true);
    try {
      const { url: newUrl } = await api.regenerateInviteCode();
      setUrl(newUrl);
      toast.success("New invite link generated — the old one no longer works");
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Could not regenerate link",
      );
    } finally {
      setRegenerating(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Invite a member</CardTitle>
        <CardDescription>
          Share your personal link to bring someone into the community — they
          skip the approval queue automatically.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground">
            Loading your invite link…
          </p>
        ) : disabled ? (
          <p className="text-sm text-muted-foreground">
            Invite links are currently disabled by the admin.
          </p>
        ) : url ? (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-2">
              <Input readOnly value={url} className="font-mono text-xs" />
              <Button
                type="button"
                variant="outline"
                className="shrink-0"
                onClick={copyLink}
              >
                Copy
              </Button>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="self-start text-muted-foreground"
              disabled={regenerating}
              onClick={regenerate}
            >
              {regenerating && <Loader2 className="size-3.5 animate-spin" />}
              Regenerate link
            </Button>
          </div>
        ) : (
          <p className="text-sm text-destructive">
            Could not load your invite link.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
