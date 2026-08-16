"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    api
      .setupStatus()
      .then(({ needs_setup }) => {
        router.replace(needs_setup ? "/setup" : "/catalog");
      })
      .catch(() => {
        router.replace("/catalog");
      });
  }, [router]);

  return null;
}
