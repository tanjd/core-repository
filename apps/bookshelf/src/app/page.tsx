"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { LandingPage } from "@/components/landing/LandingPage";

export default function Home() {
  const router = useRouter();
  const [showLanding, setShowLanding] = useState(false);

  useEffect(() => {
    // Logged-in members skip the pitch and go straight to the catalogue —
    // the landing page below is only for logged-out visitors.
    if (localStorage.getItem("bookshelf_token")) {
      router.replace("/catalog");
      return;
    }
    api
      .setupStatus()
      .then(({ needs_setup }) => {
        if (needs_setup) {
          router.replace("/setup");
        } else {
          setShowLanding(true);
        }
      })
      .catch(() => {
        setShowLanding(true);
      });
  }, [router]);

  if (!showLanding) return null;

  return <LandingPage />;
}
