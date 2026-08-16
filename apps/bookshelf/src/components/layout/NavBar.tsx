"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { BookOpen, Menu, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NotificationBell } from "@/components/NotificationBell";
import { cn } from "@/lib/utils";

const authLinks = [
  { href: "/catalog", label: "Catalog" },
  { href: "/share", label: "Share a Book" },
  { href: "/my-books", label: "My Books" },
  { href: "/my-requests", label: "My Requests" },
];

const guestLinks = [{ href: "/catalog", label: "Catalog" }];

function navLinkClass(active: boolean) {
  return cn(
    "px-3 py-1.5 rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
    active ? "bg-accent text-accent-foreground" : "text-muted-foreground",
  );
}

function mobileNavLinkClass(active: boolean) {
  return cn(
    "px-3 py-2 rounded-md text-sm font-medium transition-colors hover:bg-accent",
    active ? "bg-accent text-accent-foreground" : "text-muted-foreground",
  );
}

export function NavBar() {
  const router = useRouter();
  const pathname = usePathname();
  const [isAuth, setIsAuth] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

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
    setMobileOpen(false);
    router.push("/login");
  }

  const navLinks = isAuth ? authLinks : guestLinks;

  // For admin users the "Admin" link covers profile too (first tab is Profile).
  // For regular users a standalone "Profile" link is shown.
  const profileHref = isAdmin ? "/admin/profile" : "/profile";
  const profileActive = isAdmin
    ? pathname.startsWith("/admin")
    : pathname === "/profile";

  return (
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

          {/* Desktop nav */}
          <div className="hidden md:flex items-center gap-1">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={navLinkClass(pathname === link.href)}
              >
                {link.label}
              </Link>
            ))}

            {isAuth ? (
              <>
                <NotificationBell />
                <Link
                  href={profileHref}
                  className={navLinkClass(profileActive)}
                >
                  {isAdmin ? "Admin" : "Profile"}
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
          </div>

          {/* Mobile hamburger */}
          <button
            className="md:hidden p-2 rounded-md hover:bg-accent"
            onClick={() => setMobileOpen((v) => !v)}
            aria-label="Toggle menu"
          >
            {mobileOpen ? (
              <X className="size-5" />
            ) : (
              <Menu className="size-5" />
            )}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="md:hidden border-t bg-background px-4 py-3 flex flex-col gap-1">
          {navLinks.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              onClick={() => setMobileOpen(false)}
              className={mobileNavLinkClass(pathname === link.href)}
            >
              {link.label}
            </Link>
          ))}

          {isAuth ? (
            <>
              <div onClick={() => setMobileOpen(false)}>
                <NotificationBell />
              </div>
              <Link
                href={profileHref}
                onClick={() => setMobileOpen(false)}
                className={mobileNavLinkClass(profileActive)}
              >
                {isAdmin ? "Admin" : "Profile"}
              </Link>
              <button
                onClick={handleLogout}
                className="px-3 py-2 rounded-md text-sm font-medium text-left text-muted-foreground hover:bg-accent transition-colors"
              >
                Logout
              </button>
            </>
          ) : (
            <>
              <Link
                href="/login"
                onClick={() => setMobileOpen(false)}
                className={mobileNavLinkClass(pathname === "/login")}
              >
                Login
              </Link>
              <Link
                href="/register"
                onClick={() => setMobileOpen(false)}
                className={mobileNavLinkClass(pathname === "/register")}
              >
                Register
              </Link>
            </>
          )}
        </div>
      )}
    </nav>
  );
}
