"use client";

import { useEffect, useRef, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { InviteCode } from "@/lib/api";
import { Button } from "@/components/ui/button";

// Moved out of the Users page into its own "Users & Access" tab — see
// apps/bookshelf/docs/invite-code-spec.md ("Admins can see every member's
// link and revoke any one of them from the Users admin page"; this is that
// same view, just promoted to a peer of Users rather than a section within
// it, so an admin doesn't have to scroll past the whole member table to
// reach it).
export default function AdminInvitesPage() {
  const [inviteCodes, setInviteCodes] = useState<InviteCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<number | null>(null);

  async function loadInviteCodes() {
    setLoading(true);
    try {
      setInviteCodes(await api.adminListInviteCodes());
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Could not load invite links",
      );
    } finally {
      setLoading(false);
    }
  }

  // Guarded the same way wishlist/page.tsx's and my-books/page.tsx's mount
  // fetches are (not just an empty deps array) — React Strict Mode's
  // simulated mount→unmount→remount would otherwise double-fire the fetch.
  const loadedRef = useRef(false);
  useEffect(() => {
    if (loadedRef.current) return;
    loadedRef.current = true;
    loadInviteCodes();
  }, []);

  async function revokeInviteCode(inviteCode: InviteCode) {
    if (
      !confirm(
        `Revoke ${inviteCode.inviter_name}'s invite link? Their next visit to their profile will issue a new one.`,
      )
    )
      return;
    setRevokingId(inviteCode.id);
    try {
      await api.adminRevokeInviteCode(inviteCode.id);
      setInviteCodes((prev) => prev.filter((c) => c.id !== inviteCode.id));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Could not revoke invite link",
      );
    } finally {
      setRevokingId(null);
    }
  }

  if (loading) {
    return <p className="text-muted-foreground">Loading invite links…</p>;
  }

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-4">
        Every member&apos;s personal invite link. Revoking one takes effect
        immediately — the member&apos;s next profile visit lazily creates a
        fresh one.
      </p>
      {inviteCodes.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No members have an invite link yet.
        </p>
      ) : (
        <div className="rounded-md border overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-3 text-left font-medium">Member</th>
                <th className="px-4 py-3 text-left font-medium">Link</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
                <th className="px-4 py-3 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {inviteCodes.map((inviteCode) => (
                <tr
                  key={inviteCode.id}
                  className="border-b last:border-0 hover:bg-muted/30"
                >
                  <td className="px-4 py-3 font-medium">
                    {inviteCode.inviter_name}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground font-mono text-xs">
                    {typeof window !== "undefined"
                      ? `${window.location.origin}/register?invite=${inviteCode.code}`
                      : inviteCode.code}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(inviteCode.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive hover:text-destructive hover:bg-destructive/10"
                      disabled={revokingId === inviteCode.id}
                      onClick={() => revokeInviteCode(inviteCode)}
                    >
                      {revokingId === inviteCode.id && (
                        <Loader2 className="size-3.5 animate-spin" />
                      )}
                      Revoke
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
