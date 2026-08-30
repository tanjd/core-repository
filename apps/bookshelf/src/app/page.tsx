"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { LandingPage } from "@/components/landing/LandingPage";

type LandingState = "loading" | "authenticated" | "anonymous";

export default function Home() {
  const router = useRouter();
  const [state, setState] = useState<LandingState>("loading");

  useEffect(() => {
    async function resolveLandingState() {
      // Logged-in members can view the pitch too, just with different CTAs —
      // the setup check below only matters for logged-out first-run visitors.
      if (localStorage.getItem("bookshelf_token")) {
        setState("authenticated");
        return;
      }
      try {
        const { needs_setup } = await api.setupStatus();
        if (needs_setup) {
          router.replace("/setup");
        } else {
          setState("anonymous");
        }
      } catch {
        setState("anonymous");
      }
    }
    resolveLandingState();
  }, [router]);

  if (state === "loading") return null;

  return <LandingPage isAuthenticated={state === "authenticated"} />;
}
