"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { api } from "@/lib/api";
import type { User } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/Pagination";

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

  useEffect(() => {
    const stored = localStorage.getItem("bookshelf_user");
    if (stored) {
      try {
        setCurrentUserId(JSON.parse(stored).id);
      } catch {
        /* ignore */
      }
    }
    loadUsers(1);
  }, []);

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
                <td className="px-4 py-3">
                  <div className="flex gap-2 justify-end flex-wrap">
                    {user.id !== currentUserId &&
                      (() => {
                        const busyAction = actionLoading[user.id];
                        const isBusy = busyAction !== undefined;
                        return (
                          <>
                            {user.pending_approval && (
                              <Button
                                size="sm"
                                disabled={isBusy}
                                onClick={() => toggleApproval(user)}
                              >
                                {busyAction === "approve" && (
                                  <Loader2 className="size-3.5 animate-spin" />
                                )}
                                Approve
                              </Button>
                            )}
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={isBusy}
                              onClick={() => toggleRole(user)}
                            >
                              {busyAction === "role" && (
                                <Loader2 className="size-3.5 animate-spin" />
                              )}
                              {user.role === "admin" ? "Demote" : "Promote"}
                            </Button>
                            <Button
                              size="sm"
                              variant={user.suspended ? "outline" : "secondary"}
                              disabled={isBusy}
                              onClick={() => toggleSuspended(user)}
                            >
                              {busyAction === "suspend" && (
                                <Loader2 className="size-3.5 animate-spin" />
                              )}
                              {user.suspended ? "Unsuspend" : "Suspend"}
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              className="text-destructive hover:text-destructive hover:bg-destructive/10"
                              disabled={isBusy}
                              onClick={() => deleteUser(user)}
                            >
                              {busyAction === "delete" && (
                                <Loader2 className="size-3.5 animate-spin" />
                              )}
                              Delete
                            </Button>
                          </>
                        );
                      })()}
                  </div>
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
    </div>
  );
}
