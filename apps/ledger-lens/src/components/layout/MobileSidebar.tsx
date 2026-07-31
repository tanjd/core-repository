"use client";

import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useMobileNav } from "@/context/MobileNavContext";
import { useBackendVersion } from "@/hooks/useStatement";
import { SidebarNavContent } from "./SidebarNavContent";

export function MobileSidebar() {
  const { isOpen, closeNav } = useMobileNav();
  const { data: backendVersion } = useBackendVersion();
  const frontendVersion = process.env.NEXT_PUBLIC_VERSION ?? "dev";

  return (
    <div
      className={cn(
        "fixed inset-0 z-50 md:hidden",
        isOpen ? "block" : "hidden",
      )}
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50"
        onClick={closeNav}
        aria-hidden="true"
      />

      {/* Drawer panel */}
      <aside className="absolute inset-y-0 left-0 flex w-56 flex-col bg-sidebar border-r">
        {/* Header: logo + close button */}
        <div className="flex h-14 items-center justify-between border-b px-4">
          <span className="text-sm font-semibold tracking-tight text-sidebar-foreground">
            Ledger Lens
          </span>
          <button
            onClick={closeNav}
            className="rounded-md p-1 text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground"
            aria-label="Close navigation"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <SidebarNavContent onNavigate={closeNav} />

        {/* Footer: version info */}
        <div className="border-t p-3 space-y-0.5">
          <p className="text-xs text-muted-foreground">Portfolio Dashboard</p>
          <p className="text-xs text-muted-foreground/60">
            <span>ui v{frontendVersion}</span>
            <span className="mx-1">&middot;</span>
            <span>api v{backendVersion?.version ?? "–"}</span>
          </p>
        </div>
      </aside>
    </div>
  );
}
