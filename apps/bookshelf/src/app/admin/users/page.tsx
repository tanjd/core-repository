"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { User } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Pagination } from "@/components/ui/Pagination";

const PAGE_SIZE = 20;

export default function AdminUsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [currentUserId, setCurrentUserId] = useState<number | null>(null);

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

  async function toggleRole(user: User) {
    const newRole = user.role === "admin" ? "user" : "admin";
    const updated = await api.adminUpdateUser(user.id, { role: newRole });
    setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
  }

  async function toggleSuspended(user: User) {
    const updated = await api.adminUpdateUser(user.id, {
      suspended: !user.suspended,
    });
    setUsers((prev) => prev.map((u) => (u.id === updated.id ? updated : u)));
  }

  async function deleteUser(user: User) {
    if (!confirm(`Delete user "${user.name}"? This cannot be undone.`)) return;
    await api.adminDeleteUser(user.id);
    await loadUsers(page);
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
                    {user.id !== currentUserId && (
                      <>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => toggleRole(user)}
                        >
                          {user.role === "admin" ? "Demote" : "Promote"}
                        </Button>
                        <Button
                          size="sm"
                          variant={user.suspended ? "outline" : "secondary"}
                          onClick={() => toggleSuspended(user)}
                        >
                          {user.suspended ? "Unsuspend" : "Suspend"}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="text-destructive hover:text-destructive hover:bg-destructive/10"
                          onClick={() => deleteUser(user)}
                        >
                          Delete
                        </Button>
                      </>
                    )}
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
