"use client";

import { useEffect, useState, useRef, type ReactNode } from "react";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { BookOpen, ChevronDown, LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NotificationBell } from "@/components/NotificationBell";
import { BottomTabBar } from "@/components/layout/BottomTabBar";
import { ThemeToggle } from "@/components/theme-toggle";
import { primaryNavItems, profileNavItem } from "@/components/layout/navItems";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

function navLinkClass(active: boolean) {
  return cn(
    "px-3 py-1.5 rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
    active ? "bg-accent text-accent-foreground" : "text-muted-foreground",
  );
}

// Shared by the desktop and mobile headers: tap the trigger for a popover
// with the Profile/Admin link followed by Logout (Facebook-style), rather
// than Logout being its own always-visible control.
function ProfileMenu({
  profileItem,
  onLogout,
  trigger,
}: {
  profileItem: ReturnType<typeof profileNavItem>;
  onLogout: () => void;
  trigger: ReactNode;
}) {
  return (
    <Popover>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="end"
        sideOffset={8}
        className="w-44 p-1"
      >
        <Link
          href={profileItem.href}
          className="flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
        >
          <profileItem.icon className="size-4" />
          {profileItem.label}
        </Link>
        <button
          onClick={onLogout}
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent"
        >
          <LogOut className="size-4" />
          Logout
        </button>
      </PopoverContent>
    </Popover>
  );
}

export function NavBar() {
  const router = useRouter();
  const pathname = usePathname();
  const [isAuth, setIsAuth] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);
  const prevAuthRef = useRef<{ isAuth: boolean; isAdmin: boolean } | null>(
    null,
  );

  // Re-derived from localStorage on every pathname change (not just on
  // mount) since login/logout navigates within the same NavBar instance —
  // gated behind a ref comparison (rather than an unconditional setState) so
  // it only re-renders when the derived auth state actually changed.
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    const nextIsAuth = !!token;
    let nextIsAdmin = false;
    try {
      const stored = localStorage.getItem("bookshelf_user");
      const user = stored ? JSON.parse(stored) : null;
      nextIsAdmin = user?.role === "admin";
    } catch {
      nextIsAdmin = false;
    }
    const prev = prevAuthRef.current;
    if (!prev || prev.isAuth !== nextIsAuth || prev.isAdmin !== nextIsAdmin) {
      prevAuthRef.current = { isAuth: nextIsAuth, isAdmin: nextIsAdmin };
      setIsAuth(nextIsAuth);
      setIsAdmin(nextIsAdmin);
    }
  }, [pathname]);

  function handleLogout() {
    localStorage.removeItem("bookshelf_token");
    localStorage.removeItem("bookshelf_user");
    setIsAuth(false);
    setIsAdmin(false);
    router.push("/login");
  }

  const profileItem = profileNavItem(isAdmin);

  return (
    <>
      <nav className="sticky top-0 z-50 border-b bg-background/95 backdrop-blur">
        <div className="max-w-6xl mx-auto px-4">
          <div className="flex h-14 items-center justify-between">
            {/* Brand */}
            <Link
              href={isAuth ? "/catalog" : "/"}
              className="flex items-center gap-2 font-semibold text-lg"
            >
              <BookOpen className="size-5" />
              Bookshelf
            </Link>

            {/* Desktop nav — primary navigation lives in the bottom tab bar
                on mobile instead, see BottomTabBar */}
            <div className="hidden md:flex items-center gap-1">
              {isAuth ? (
                <>
                  {primaryNavItems.map((item) => (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={navLinkClass(item.isActive(pathname))}
                    >
                      {item.label}
                    </Link>
                  ))}
                  <NotificationBell />
                  <ProfileMenu
                    profileItem={profileItem}
                    onLogout={handleLogout}
                    trigger={
                      <button
                        className={cn(
                          navLinkClass(profileItem.isActive(pathname)),
                          "flex items-center gap-1",
                        )}
                      >
                        {profileItem.label}
                        <ChevronDown className="size-3.5" />
                      </button>
                    }
                  />
                </>
              ) : (
                <>
                  <Link
                    href="/login"
                    className={navLinkClass(pathname === "/login")}
                  >
                    Login
                  </Link>
                  <Link href="/register">
                    <Button size="sm">Register</Button>
                  </Link>
                </>
              )}
              <ThemeToggle />
            </div>

            {/* Mobile — guests get compact login/register CTAs; authenticated
                users get Alerts, theme toggle, and a Profile menu (Facebook-
                style: tap the profile icon for a popover with Profile/Admin
                + Logout) since navigation moved to the bottom tab bar */}
            <div className="md:hidden flex items-center gap-1">
              {isAuth ? (
                <>
                  <NotificationBell />
                  <ThemeToggle />
                  <ProfileMenu
                    profileItem={profileItem}
                    onLogout={handleLogout}
                    trigger={
                      <button
                        className="p-2 rounded-md text-muted-foreground hover:bg-accent transition-colors"
                        aria-label="Profile menu"
                      >
                        <profileItem.icon className="size-5" />
                      </button>
                    }
                  />
                </>
              ) : (
                <>
                  <Link
                    href="/login"
                    className={navLinkClass(pathname === "/login")}
                  >
                    Login
                  </Link>
                  <Link href="/register">
                    <Button size="sm">Register</Button>
                  </Link>
                  <ThemeToggle />
                </>
              )}
            </div>
          </div>
        </div>
      </nav>

      {isAuth && <BottomTabBar />}
    </>
  );
}
