import type { ComponentType } from "react";
import {
  BookOpen,
  PlusCircle,
  Library,
  ListChecks,
  ShieldCheck,
  UserRound,
} from "lucide-react";

export type NavItem = {
  href: string;
  label: string;
  shortLabel?: string;
  icon: ComponentType<{ className?: string }>;
  isActive: (pathname: string) => boolean;
  // Desktop has room for every destination in the top nav; the mobile bottom
  // tab bar doesn't, so items can opt out (e.g. "Share" moves to a FAB on
  // Catalog instead of taking a thumb-reachable tab slot).
  mobileTab?: boolean;
};

// Shared by NavBar (desktop top nav) and BottomTabBar (mobile) so there's
// one place to add/rename/remove a primary destination.
export const primaryNavItems: NavItem[] = [
  {
    href: "/catalog",
    label: "Catalog",
    icon: BookOpen,
    isActive: (p) => p === "/catalog",
  },
  {
    href: "/share",
    label: "Share a Book",
    shortLabel: "Share",
    icon: PlusCircle,
    isActive: (p) => p === "/share",
    mobileTab: false,
  },
  {
    href: "/my-books",
    label: "My Books",
    shortLabel: "Books",
    icon: Library,
    isActive: (p) => p === "/my-books",
  },
  {
    href: "/my-requests",
    label: "My Requests",
    shortLabel: "Requests",
    icon: ListChecks,
    isActive: (p) => p === "/my-requests",
  },
];

// For admin users the "Admin" link covers profile too; regular users get a
// standalone "Profile" link.
export function profileNavItem(isAdmin: boolean): NavItem {
  return {
    href: isAdmin ? "/admin/profile" : "/profile",
    label: isAdmin ? "Admin" : "Profile",
    icon: isAdmin ? ShieldCheck : UserRound,
    isActive: (p) => (isAdmin ? p.startsWith("/admin") : p === "/profile"),
  };
}
