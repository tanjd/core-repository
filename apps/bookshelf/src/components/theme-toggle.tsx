"use client";

import { useEffect, useState, useRef } from "react";
import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  const mountedRef = useRef(false);

  // Theme isn't known until after hydration (it depends on localStorage/OS
  // preference), so render a disabled placeholder first to avoid a
  // server/client mismatch flash.
  useEffect(() => {
    if (mountedRef.current) return;
    mountedRef.current = true;
    setMounted(true);
  }, []);

  if (!mounted) {
    return (
      <Button
        variant="ghost"
        size="icon"
        disabled
        aria-label="Toggle theme"
        className="text-muted-foreground"
      >
        <Sun className="size-5" />
      </Button>
    );
  }

  const isDark = resolvedTheme === "dark";

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label={isDark ? "Switch to light mode" : "Switch to dark mode"}
      className="text-muted-foreground"
    >
      {isDark ? <Sun className="size-5" /> : <Moon className="size-5" />}
    </Button>
  );
}
