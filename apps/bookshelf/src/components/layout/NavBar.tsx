"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { BookOpen, LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NotificationBell } from "@/components/NotificationBell";
import { BottomTabBar } from "@/components/layout/BottomTabBar";
import { ThemeToggle } from "@/components/theme-toggle";
import { primaryNavItems, profileNavItem } from "@/components/layout/navItems";
import { cn } from "@/lib/utils";

function navLinkClass(active: boolean) {
  return cn(
    "px-3 py-1.5 rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
    active ? "bg-accent text-accent-foreground" : "text-muted-foreground",
  );
}

export function NavBar() {
  const router = useRouter();
  const pathname = usePathname();
  const [isAuth, setIsAuth] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    setIsAuth(!!token);
    try {
      const stored = localStorage.getItem("bookshelf_user");
      const user = stored ? JSON.parse(stored) : null;
      setIsAdmin(user?.role === "admin");
    } catch {
      setIsAdmin(false);
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
              href="/catalog"
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
                  <Link
                    href={profileItem.href}
                    className={navLinkClass(profileItem.isActive(pathname))}
                  >
                    {profileItem.label}
                  </Link>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleLogout}
                    className="text-muted-foreground"
                  >
                    Logout
                  </Button>
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
                users just get a logout icon since navigation moved to the
                bottom tab bar */}
            <div className="md:hidden flex items-center gap-1">
              {isAuth ? (
                <button
                  onClick={handleLogout}
                  className="p-2 rounded-md text-muted-foreground hover:bg-accent transition-colors"
                  aria-label="Logout"
                >
                  <LogOut className="size-5" />
                </button>
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
          </div>
        </div>
      </nav>

      {isAuth && <BottomTabBar isAdmin={isAdmin} />}
    </>
  );
}
