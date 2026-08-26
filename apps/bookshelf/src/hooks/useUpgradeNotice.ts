"use client";

import { useCallback, useState } from "react";

import { semverGt } from "@/lib/changelog";

const APP_KEY = "bookshelf_last_seen_app_version";

function readStoredVersion(): string | null {
  try {
    return localStorage.getItem(APP_KEY);
  } catch {
    return null;
  }
}

function readUpgradeVisible(currentVersion: string): boolean {
  const stored = readStoredVersion();
  if (stored === null) {
    return false;
  }
  return semverGt(currentVersion, stored);
}

export function useUpgradeNotice() {
  const currentVersion = process.env.NEXT_PUBLIC_VERSION ?? "0.0.0";
  const [visible, setVisible] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return readUpgradeVisible(currentVersion);
  });

  const dismiss = useCallback(() => {
    try {
      localStorage.setItem(APP_KEY, currentVersion);
    } catch {
      // ignore quota / private-mode failures
    }
    setVisible(false);
  }, [currentVersion]);

  return {
    visible,
    version: currentVersion,
    dismiss,
  };
}
