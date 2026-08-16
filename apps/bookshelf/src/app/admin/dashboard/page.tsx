"use client";

import { useEffect, useState, useCallback } from "react";
import {
  BookOpen,
  Library,
  Users,
  UserPlus,
  AlertTriangle,
  RefreshCw,
} from "lucide-react";
import { api } from "@/lib/api";
import type { DashboardStats } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

function StatCard({
  icon: Icon,
  label,
  value,
  detail,
  warn,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: number;
  detail?: string;
  warn?: boolean;
}) {
  return (
    <Card className="py-4 gap-2">
      <CardContent className="px-4 flex flex-col gap-1.5">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Icon className="size-4" />
          <span className="text-xs font-medium">{label}</span>
        </div>
        <p
          className={cn(
            "text-2xl font-bold",
            warn && value > 0 && "text-destructive",
          )}
        >
          {value}
        </p>
        {detail && <p className="text-xs text-muted-foreground">{detail}</p>}
      </CardContent>
    </Card>
  );
}

export default function AdminDashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadStats = useCallback(async () => {
    try {
      const data = await api.adminGetDashboardStats();
      setStats(data);
      setError("");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load dashboard stats",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="h-24 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
        <div className="h-48 rounded-lg bg-muted animate-pulse" />
      </div>
    );
  }

  if (error || !stats) {
    return (
      <div className="flex flex-col gap-3">
        <p className="text-sm text-destructive">
          {error || "Failed to load dashboard stats"}
        </p>
        <Button
          variant="outline"
          size="sm"
          onClick={loadStats}
          className="self-start"
        >
          <RefreshCw className="size-3.5" /> Retry
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-end -mb-2">
        <Button variant="ghost" size="sm" onClick={loadStats}>
          <RefreshCw className="size-3.5" /> Refresh
        </Button>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
        <StatCard
          icon={BookOpen}
          label="Books"
          value={stats.total_books}
          detail={`${stats.total_copies} cop${stats.total_copies === 1 ? "y" : "ies"}`}
        />
        <StatCard
          icon={Library}
          label="Available copies"
          value={stats.available_copies}
          detail={`${stats.loaned_copies} loaned out`}
        />
        <StatCard icon={Users} label="Members" value={stats.total_users} />
        <StatCard
          icon={UserPlus}
          label="Signups this week"
          value={stats.signups_this_week}
        />
        <StatCard
          icon={AlertTriangle}
          label="Overdue loans"
          value={stats.overdue_count}
          warn
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="rounded-md border overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-3 text-left font-medium" colSpan={2}>
                  Most borrowed books
                </th>
              </tr>
            </thead>
            <tbody>
              {stats.most_borrowed_books.length === 0 ? (
                <tr>
                  <td
                    className="px-4 py-6 text-center text-muted-foreground"
                    colSpan={2}
                  >
                    No loans yet.
                  </td>
                </tr>
              ) : (
                stats.most_borrowed_books.map((b) => (
                  <tr
                    key={b.book_id}
                    className="border-b last:border-0 hover:bg-muted/30"
                  >
                    <td className="px-4 py-3">
                      <p className="font-medium">{b.title}</p>
                      <p className="text-xs text-muted-foreground">
                        {b.author}
                      </p>
                    </td>
                    <td className="px-4 py-3 text-right text-muted-foreground whitespace-nowrap">
                      {b.borrow_count} loan{b.borrow_count !== 1 ? "s" : ""}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="rounded-md border overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-3 text-left font-medium" colSpan={2}>
                  Active lenders
                </th>
              </tr>
            </thead>
            <tbody>
              {stats.active_lenders.length === 0 ? (
                <tr>
                  <td
                    className="px-4 py-6 text-center text-muted-foreground"
                    colSpan={2}
                  >
                    No copies currently out on loan.
                  </td>
                </tr>
              ) : (
                stats.active_lenders.map((l) => (
                  <tr
                    key={l.user_id}
                    className="border-b last:border-0 hover:bg-muted/30"
                  >
                    <td className="px-4 py-3 font-medium">{l.name}</td>
                    <td className="px-4 py-3 text-right text-muted-foreground whitespace-nowrap">
                      {l.active_loans} out
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
