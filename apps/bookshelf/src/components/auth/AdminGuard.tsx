"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

export function AdminGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token");
    if (!token) {
      router.push("/login");
      return;
    }
    try {
      const stored = localStorage.getItem("bookshelf_user");
      const user = stored ? JSON.parse(stored) : null;
      if (user?.role !== "admin") {
        router.push("/catalog");
        return;
      }
    } catch {
      router.push("/catalog");
      return;
    }
    setChecked(true);
  }, [router]);

  if (!checked) return null;

  return <>{children}</>;
}
