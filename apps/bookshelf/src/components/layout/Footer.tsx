import Link from "next/link";

import { getAppVersion } from "@/lib/changelog";

export function Footer() {
  return (
    <footer className="shrink-0 text-sm text-muted-foreground text-center py-6 pb-24 md:pb-6 flex items-center justify-center gap-3">
      <Link href="/changelog" className="hover:underline underline-offset-2">
        v{getAppVersion()}
      </Link>
      <span>·</span>
      <a href="/about" className="hover:underline underline-offset-2">
        About
      </a>
    </footer>
  );
}
