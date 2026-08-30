"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

// Auth check mirrors NavBar.tsx's — re-derived from localStorage on every
// pathname change (not just on mount) since login/logout navigates within
// the same layout instance, gated behind a ref comparison so it only
// re-renders when the derived auth state actually changed.
export function Footer() {
  const pathname = usePathname();
  const [isAuth, setIsAuth] = useState(false);
  const prevAuthRef = useRef<boolean | null>(null);

  useEffect(() => {
    const nextIsAuth = !!localStorage.getItem("bookshelf_token");
    if (prevAuthRef.current !== nextIsAuth) {
      prevAuthRef.current = nextIsAuth;
      setIsAuth(nextIsAuth);
    }
  }, [pathname]);

  return (
    <footer className="shrink-0 text-sm text-muted-foreground text-center py-6 pb-24 md:pb-6 flex items-center justify-center gap-3">
      <Link href="/changelog" className="hover:underline underline-offset-2">
        v{process.env.NEXT_PUBLIC_VERSION}
      </Link>
      <span>·</span>
      <a href="/about" className="hover:underline underline-offset-2">
        About
      </a>
      {isAuth && (
        <>
          <span>·</span>
          <Link
            href="/profile#invite-a-member"
            className="hover:underline underline-offset-2"
          >
            Invite a member
          </Link>
        </>
      )}
    </footer>
  );
}
