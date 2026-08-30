"use client";

import { useEffect, useRef, useState } from "react";
import { Loader2, MoreVertical, Search, X } from "lucide-react";
import { api } from "@/lib/api";
import type { User } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/ui/Pagination";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const PAGE_SIZE = 20;

type UserAction = "approve" | "role" | "suspend" | "delete";
type RoleFilter = "all" | "user" | "admin";
type StatusFilter =
  | "all"
  | "verified"
  | "unverified"
  | "pending_approval"
  | "suspended";

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

  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<RoleFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const hasActiveFilters =
    !!search.trim() || roleFilter !== "all" || statusFilter !== "all";

  function clearFilters() {
    setSearch("");
    setRoleFilter("all");
    setStatusFilter("all");
  }

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
    // Only the identity-lookup half of this effect is mount-only; loadUsers
    // itself is re-triggered by the debounced filter effect below, same
    // split as catalog/page.tsx's mount vs. debounce effects.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Debounced search/role/status — mirrors catalog/page.tsx's pattern:
  // skip the first (mount) run since the effect above already fetched page
  // 1, then debounce so fast typing doesn't fire a request per keystroke.
  const mountedRef = useRef(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    mountedRef.current = false;
  }, []);
  useEffect(() => {
    if (!mountedRef.current) {
      mountedRef.current = true;
      return;
    }
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      loadUsers(1);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, roleFilter, statusFilter]);

  async function loadUsers(p: number) {
    setLoading(true);
    try {
      const data = await api.adminListUsers({
        page: p,
        page_size: PAGE_SIZE,
        search: search.trim() || undefined,
        role: roleFilter === "all" ? undefined : roleFilter,
        status: statusFilter === "all" ? undefined : statusFilter,
      });
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

  return (
    <div>
      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center mb-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input
            type="search"
            placeholder="Search by name or email…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9 h-9 pr-9"
          />
          {search && (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setSearch("")}
              className="absolute right-2 top-1/2 -translate-y-1/2 flex size-6 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          )}
        </div>

        <Select
          value={roleFilter}
          onValueChange={(v) => setRoleFilter(v as RoleFilter)}
        >
          <SelectTrigger className="h-9 w-32">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All roles</SelectItem>
            <SelectItem value="user">User</SelectItem>
            <SelectItem value="admin">Admin</SelectItem>
          </SelectContent>
        </Select>

        <Select
          value={statusFilter}
          onValueChange={(v) => setStatusFilter(v as StatusFilter)}
        >
          <SelectTrigger className="h-9 w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="verified">Verified</SelectItem>
            <SelectItem value="unverified">Unverified</SelectItem>
            <SelectItem value="pending_approval">Pending approval</SelectItem>
            <SelectItem value="suspended">Suspended</SelectItem>
          </SelectContent>
        </Select>

        {hasActiveFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Clear filters
          </Button>
        )}
      </div>

      <p className="text-sm text-muted-foreground mb-4">
        {total} user{total !== 1 ? "s" : ""}
      </p>

      {loading ? (
        <p className="text-muted-foreground">Loading users…</p>
      ) : (
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
              {users.length === 0 ? (
                <tr>
                  <td
                    colSpan={7}
                    className="px-4 py-6 text-center text-muted-foreground"
                  >
                    {hasActiveFilters
                      ? "No users match your search/filters."
                      : "No users yet."}
                  </td>
                </tr>
              ) : (
                users.map((user) => (
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
                        variant={
                          user.role === "admin" ? "default" : "secondary"
                        }
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
                                <DropdownMenuItem
                                  onClick={() => toggleRole(user)}
                                >
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
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
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
