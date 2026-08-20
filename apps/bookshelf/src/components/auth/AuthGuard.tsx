"use client";

import { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";

// Gates a route behind login, mirroring AdminGuard's pattern minus the role
// check — used by src/app/catalog/layout.tsx so the catalog (previously
// browsable while logged out) requires an account like every other member
// page.
export function AuthGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [checked, setChecked] = useState(false);
  const checkedRef = useRef(false);

  useEffect(() => {
    if (checkedRef.current) return;
    checkedRef.current = true;
    const token = localStorage.getItem("bookshelf_token");
    const stored = token ? localStorage.getItem("bookshelf_user") : null;
    let validUser = false;
    if (stored) {
      try {
        JSON.parse(stored);
        validUser = true;
      } catch {
        validUser = false;
      }
    }
    if (!token || !validUser) {
      router.push("/login");
      return;
    }
    setChecked(true);
  }, [router]);

  if (!checked) return null;

  return <>{children}</>;
}
