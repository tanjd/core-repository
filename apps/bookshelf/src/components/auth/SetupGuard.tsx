"use client";

import { useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { api } from "@/lib/api";

export function SetupGuard({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (pathname === "/setup") {
      setChecked(true);
      return;
    }
    api
      .setupStatus()
      .then(({ needs_setup }) => {
        if (needs_setup) {
          router.replace("/setup");
        } else {
          setChecked(true);
        }
      })
      .catch(() => {
        setChecked(true);
      });
  }, [pathname, router]);

  if (!checked && pathname !== "/setup") return null;
  return <>{children}</>;
}
