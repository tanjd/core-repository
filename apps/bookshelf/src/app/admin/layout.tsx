"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { AdminGuard } from "@/components/auth/AdminGuard";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";

const adminLinks = [
  { href: "/admin/dashboard", label: "Dashboard" },
  { href: "/admin/profile", label: "Profile" },
  { href: "/admin/users", label: "Users" },
  { href: "/admin/settings", label: "Settings" },
  { href: "/admin/announcements", label: "Announcements" },
  { href: "/admin/jobs", label: "Jobs" },
  { href: "/admin/backups", label: "Backups" },
  { href: "/admin/metadata", label: "Metadata" },
];

const navGroups = [
  {
    label: "Overview",
    links: [{ href: "/admin/dashboard", label: "Dashboard" }],
  },
  {
    label: "Content",
    links: [
      { href: "/admin/announcements", label: "Announcements" },
      { href: "/admin/metadata", label: "Metadata" },
    ],
  },
  {
    label: "Users & Access",
    links: [
      { href: "/admin/users", label: "Users" },
      { href: "/admin/settings", label: "Settings" },
    ],
  },
  {
    label: "Operations",
    links: [
      { href: "/admin/jobs", label: "Jobs" },
      { href: "/admin/backups", label: "Backups" },
    ],
  },
];

const profileLink = { href: "/admin/profile", label: "Profile" };

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => {
    api
      .adminGetDashboardStats()
      .then((stats) => setPendingCount(stats.pending_approval_count))
      .catch(() => {
        /* nav badge is a nice-to-have; ignore load failures */
      });
  }, []);

  function NavBadge({ href }: { href: string }) {
    if (href !== "/admin/users" || pendingCount <= 0) return null;
    return (
      <span className="inline-flex min-w-4 h-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold text-primary-foreground">
        {pendingCount}
      </span>
    );
  }

  return (
    <AdminGuard>
      <div className="max-w-6xl mx-auto px-4 py-6">
        <h1 className="text-2xl font-bold mb-4">Admin</h1>

        {/* Mobile: flat scrollable tab strip */}
        <nav className="md:hidden mb-6 flex gap-2 overflow-x-auto border-b pb-2">
          {adminLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={cn(
                "shrink-0 whitespace-nowrap flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors hover:bg-accent",
                pathname === link.href
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground",
              )}
            >
              {link.label}
              <NavBadge href={link.href} />
            </Link>
          ))}
        </nav>

        <div className="md:flex md:items-start md:gap-8">
          {/* Desktop: grouped sidebar */}
          <aside className="hidden md:flex md:w-56 md:shrink-0 md:flex-col">
            {navGroups.map((group) => (
              <div key={group.label} className="mb-5">
                <p className="mb-1.5 px-2.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                  {group.label}
                </p>
                {group.links.map((link) => (
                  <Link
                    key={link.href}
                    href={link.href}
                    className={cn(
                      "mb-0.5 flex items-center justify-between gap-2 rounded-md px-2.5 py-2 text-sm font-medium transition-colors hover:bg-accent",
                      pathname === link.href
                        ? "bg-accent text-accent-foreground"
                        : "text-muted-foreground",
                    )}
                  >
                    <span>{link.label}</span>
                    <NavBadge href={link.href} />
                  </Link>
                ))}
              </div>
            ))}
            <div className="mt-auto border-t pt-3">
              <Link
                href={profileLink.href}
                className={cn(
                  "flex items-center rounded-md px-2.5 py-2 text-sm font-medium transition-colors hover:bg-accent",
                  pathname === profileLink.href
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground",
                )}
              >
                {profileLink.label}
              </Link>
            </div>
          </aside>

          <div className="min-w-0 flex-1">{children}</div>
        </div>
      </div>
    </AdminGuard>
  );
}
