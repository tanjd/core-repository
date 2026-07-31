"use client";

import { useBackendVersion } from "@/hooks/useStatement";
import { SidebarNavContent } from "./SidebarNavContent";

export function Sidebar() {
  const { data: backendVersion } = useBackendVersion();
  const frontendVersion = process.env.NEXT_PUBLIC_VERSION ?? "dev";

  return (
    <aside className="hidden md:flex flex-col h-full w-56 border-r bg-sidebar">
      <div className="flex h-14 items-center border-b px-4">
        <span className="text-sm font-semibold tracking-tight text-sidebar-foreground">
          Ledger Lens
        </span>
      </div>

      <SidebarNavContent />

      <div className="border-t p-3 space-y-0.5">
        <p className="text-xs text-muted-foreground">Portfolio Dashboard</p>
        <p className="text-xs text-muted-foreground/60">
          <span>ui v{frontendVersion}</span>
          <span className="mx-1">&middot;</span>
          <span>api v{backendVersion?.version ?? "–"}</span>
        </p>
      </div>
    </aside>
  );
}
