"use client";

import { useEffect, useState, useRef } from "react";
import { Loader2, MoreVertical } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { User, InviteCode } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/Pagination";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const PAGE_SIZE = 20;

type UserAction = "approve" | "role" | "suspend" | "delete";

export default function AdminUsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [currentUserId, setCurrentUserId] = useState<number | null>(null);
  const [actionLoading, setActionLoading] = useState<
    Record<number, UserAction | undefined>
  >({});

  const [inviteCodes, setInviteCodes] = useState<InviteCode[]>([]);
  const [inviteCodesLoading, setInviteCodesLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<number | null>(null);

  const identifiedRef = useRef(false);
  useEffect(() => {
    if (!identifiedRef.current) {
      identifiedRef.current = true;
      const stored = localStorage.getItem("bookshelf_user");
      let userId: number | null = null;
      if (stored) {
        try {
          userId = JSON.parse(stored).id;
        } catch {
          /* ignore */
        }
      }
      setCurrentUserId(userId);
    }
    loadUsers(1);
    loadInviteCodes();
  }, []);

  async function loadInviteCodes() {
    setInviteCodesLoading(true);
    try {
      setInviteCodes(await api.adminListInviteCodes());
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Could not load invite links",
      );
    } finally {
      setInviteCodesLoading(false);
    }
  }

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

  async function loadUsers(p: number) {
    setLoading(true);
    try {
      const data = await api.adminListUsers({ page: p, page_size: PAGE_SIZE });
      setUsers(data.items);
      setTotalPages(data.total_pages);
      setTotal(Number(data.total));
      setPage(p);
    } finally {
      setLoading(false);
    }
  }

  async function runAction(
    user: User,
    action: UserAction,
    fn: () => Promise<void>,
  ) {
    setActionLoading((prev) => ({ ...prev, [user.id]: action }));
    try {
      await fn();
    } finally {
      setActionLoading((prev) => ({ ...prev, [user.id]: undefined }));
    }
  }

  function toggleRole(user: User) {
    return runAction(user, "role", async () => {
      const newRole = user.role === "admin" ? "user" : "admin";
      const updated = await api.adminUpdateUser(user.id, { role: newRole });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
    });
  }

  function toggleSuspended(user: User) {
    return runAction(user, "suspend", async () => {
      const updated = await api.adminUpdateUser(user.id, {
        suspended: !user.suspended,
      });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
    });
  }

  function toggleApproval(user: User) {
    return runAction(user, "approve", async () => {
      const updated = await api.adminUpdateUser(user.id, {
        pending_approval: !user.pending_approval,
      });
      setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
    });
  }

  function deleteUser(user: User) {
    if (!confirm(`Delete user "${user.name}"? This cannot be undone.`)) return;
    return runAction(user, "delete", async () => {
      await api.adminDeleteUser(user.id);
      await loadUsers(page);
    });
  }

  if (loading) return <p className="text-muted-foreground">Loading users…</p>;

  return (
    <div>
      <p className="text-sm text-muted-foreground mb-4">
        {total} user{total !== 1 ? "s" : ""}
      </p>
      <div className="rounded-md border overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-4 py-3 text-left font-medium">Name</th>
              <th className="px-4 py-3 text-left font-medium">Email</th>
              <th className="px-4 py-3 text-left font-medium">Role</th>
              <th className="px-4 py-3 text-left font-medium">Status</th>
              <th className="px-4 py-3 text-left font-medium">Joined</th>
              <th className="px-4 py-3 text-left font-medium">Invited by</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.map((user) => (
              <tr
                key={user.id}
                className={`border-b last:border-0 hover:bg-muted/30 ${user.suspended ? "opacity-60" : ""}`}
              >
                <td className="px-4 py-3 font-medium">{user.name}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {user.email}
                </td>
                <td className="px-4 py-3">
                  <Badge
                    variant={user.role === "admin" ? "default" : "secondary"}
                  >
                    {user.role}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-col gap-1">
                    <Badge variant={user.verified ? "success" : "outline"}>
                      {user.verified ? "verified" : "unverified"}
                    </Badge>
                    {user.pending_approval && (
                      <Badge variant="secondary">pending approval</Badge>
                    )}
                    {user.suspended && (
                      <Badge variant="destructive">suspended</Badge>
                    )}
                  </div>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {new Date(user.created_at).toLocaleDateString()}
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {user.invited_by?.name ?? "—"}
                </td>
                <td className="px-4 py-3 text-right">
                  {user.id !== currentUserId &&
                    (() => {
                      const busyAction = actionLoading[user.id];
                      const isBusy = busyAction !== undefined;
                      return (
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button
                              size="icon-sm"
                              variant="ghost"
                              disabled={isBusy}
                              aria-label={`Actions for ${user.name}`}
                            >
                              {isBusy ? (
                                <Loader2 className="size-3.5 animate-spin" />
                              ) : (
                                <MoreVertical className="size-4" />
                              )}
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent>
                            {user.pending_approval && (
                              <DropdownMenuItem
                                onClick={() => toggleApproval(user)}
                              >
                                Approve
                              </DropdownMenuItem>
                            )}
                            <DropdownMenuItem onClick={() => toggleRole(user)}>
                              {user.role === "admin" ? "Demote" : "Promote"}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => toggleSuspended(user)}
                            >
                              {user.suspended ? "Unsuspend" : "Suspend"}
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              onClick={() => deleteUser(user)}
                            >
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      );
                    })()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {totalPages > 1 && (
        <div className="mt-4">
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={loadUsers}
          />
        </div>
      )}

      <section aria-label="Invite links" className="mt-8">
        <h2 className="text-lg font-semibold mb-2">Invite links</h2>
        <p className="text-sm text-muted-foreground mb-4">
          Every member&apos;s personal invite link. Revoking one takes effect
          immediately — the member&apos;s next profile visit lazily creates a
          fresh one.
        </p>
        {inviteCodesLoading ? (
          <p className="text-sm text-muted-foreground">Loading invite links…</p>
        ) : inviteCodes.length === 0 ? (
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
      </section>
    </div>
  );
}
