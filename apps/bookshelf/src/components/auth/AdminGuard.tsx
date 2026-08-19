"use client";

import { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";

export function AdminGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [checked, setChecked] = useState(false);
  const checkedRef = useRef(false);

  useEffect(() => {
    if (checkedRef.current) return;
    checkedRef.current = true;
    const token = localStorage.getItem("bookshelf_token");
    let isAdmin = false;
    if (token) {
      try {
        const stored = localStorage.getItem("bookshelf_user");
        const user = stored ? JSON.parse(stored) : null;
        isAdmin = user?.role === "admin";
      } catch {
        isAdmin = false;
      }
    }
    if (!token || !isAdmin) {
      router.push(token ? "/catalog" : "/login");
      return;
    }
    setChecked(true);
  }, [router]);

  if (!checked) return null;

  return <>{children}</>;
}
