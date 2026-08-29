"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { AppSetting } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";

interface SettingMeta {
  label: string;
  description: string;
  type: "bool" | "number";
  min?: number;
  max?: number;
}

const SETTING_LABELS: Record<string, SettingMeta> = {
  allow_registration: {
    label: "Allow Registration",
    description: "Whether new users can sign up",
    type: "bool",
  },
  require_registration_approval: {
    label: "Require Admin Approval for New Members",
    description:
      "New sign-ups are created but can't log in until an admin approves them from the Users page. Useful while running in beta.",
    type: "bool",
  },
  require_verified_to_borrow: {
    label: "Require Verified Email to Borrow",
    description: "Only users with a verified email can request to borrow books",
    type: "bool",
  },
  verification_requires_phone: {
    label: "Require Phone Number to Borrow",
    description: "Users must have a phone number set before they can borrow",
    type: "bool",
  },
  verification_min_books_shared: {
    label: "Min Books Shared to Borrow",
    description:
      "Users must have shared at least this many books before borrowing (0 = disabled)",
    type: "number",
  },
  max_active_loans: {
    label: "Max Active Loans Per User",
    description: "Maximum concurrent borrows per user (0 = unlimited)",
    type: "number",
  },
  max_copies_per_user: {
    label: "Max Copies Per User",
    description:
      "Maximum number of book copies a user can share (0 = unlimited)",
    type: "number",
  },
  require_email_confirmation_on_change: {
    label: "Confirm Email Changes via OTP",
    description:
      "Require users to verify ownership of a new email address (via a code sent to it) before it replaces their current one. Recommended to stay OFF until SMTP delivery is confirmed reliable in this deployment.",
    type: "bool",
  },
  monthly_digest_send_day: {
    label: "Send Day",
    description:
      "Day of the month (1-28) the monthly digest goes out to opted-in members.",
    type: "number",
    min: 1,
    max: 28,
  },
  monthly_digest_new_books_limit: {
    label: "New Books Limit",
    description: "Maximum new-book entries shown in the digest.",
    type: "number",
    min: 1,
  },
  monthly_digest_top_recommended_limit: {
    label: "Top Recommended Limit",
    description: "Maximum top-recommended-book entries shown in the digest.",
    type: "number",
    min: 1,
  },
};

// Grouped the same way Sonarr/Jellyfin group related settings under one
// section rather than one flat, unrelated list — each group renders as its
// own card, matching the status-card pattern used by the Jobs/Backups/
// Metadata admin pages.
const SETTING_GROUPS: {
  title: string;
  description?: string;
  keys: string[];
}[] = [
  {
    title: "Access & Registration",
    keys: ["allow_registration", "require_registration_approval"],
  },
  {
    title: "Borrowing Eligibility",
    keys: [
      "require_verified_to_borrow",
      "verification_requires_phone",
      "verification_min_books_shared",
      "max_active_loans",
      "max_copies_per_user",
    ],
  },
  {
    title: "Account Security",
    keys: ["require_email_confirmation_on_change"],
  },
  {
    title: "Monthly Digest",
    description:
      "Use the Enabled switch on the Jobs page to turn the whole feature off. These settings only shape what an active digest contains.",
    keys: [
      "monthly_digest_send_day",
      "monthly_digest_new_books_limit",
      "monthly_digest_top_recommended_limit",
    ],
  },
];

export default function AdminSettingsPage() {
  const [settings, setSettings] = useState<AppSetting[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    api
      .adminGetSettings()
      .then((data) => {
        setSettings(data);
        const map: Record<string, string> = {};
        data.forEach((s) => (map[s.key] = s.value));
        setValues(map);
      })
      .finally(() => setLoading(false));
  }, []);

  async function handleSave() {
    setSaving(true);
    setSaved(false);
    try {
      const updated = await api.adminUpdateSettings(
        Object.entries(values).map(([key, value]) => ({ key, value })),
      );
      setSettings(updated);
      const map: Record<string, string> = {};
      updated.forEach((s) => (map[s.key] = s.value));
      setValues(map);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } finally {
      setSaving(false);
    }
  }

  async function handleExport() {
    setExporting(true);
    try {
      const { content } = await api.adminExportSettings();
      const blob = new Blob([content], { type: "application/yaml" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "bookshelf.yaml";
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error("Failed to export settings");
    } finally {
      setExporting(false);
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-32 rounded-lg" />
        ))}
      </div>
    );
  }

  const known = new Set(settings.map((s) => s.key));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3">
        {SETTING_GROUPS.map((group) => {
          const keys = group.keys.filter((k) => known.has(k));
          if (keys.length === 0) return null;
          return (
            <div
              key={group.title}
              className="rounded-lg border bg-card p-4 flex flex-col gap-3"
            >
              <div>
                <p className="font-medium text-sm">{group.title}</p>
                {group.description && (
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {group.description}
                  </p>
                )}
              </div>
              {keys.map((key, i) => {
                const meta = SETTING_LABELS[key];
                return (
                  <div
                    key={key}
                    className={
                      i === 0
                        ? "flex items-start justify-between gap-4"
                        : "flex items-start justify-between gap-4 border-t pt-3"
                    }
                  >
                    <div className="min-w-0">
                      <p className="text-sm">{meta.label}</p>
                      {meta.description && (
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {meta.description}
                        </p>
                      )}
                    </div>
                    <div className="shrink-0">
                      {meta.type === "bool" ? (
                        <Switch
                          checked={values[key] === "true"}
                          onCheckedChange={(checked) =>
                            setValues((prev) => ({
                              ...prev,
                              [key]: checked ? "true" : "false",
                            }))
                          }
                          aria-label={meta.label}
                        />
                      ) : (
                        <Input
                          type="number"
                          min={meta.min ?? 0}
                          max={meta.max}
                          value={values[key] ?? ""}
                          onChange={(e) =>
                            setValues((prev) => ({
                              ...prev,
                              [key]: e.target.value,
                            }))
                          }
                          className="h-8 w-20 text-sm text-right"
                        />
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          );
        })}
      </div>

      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? "Saving…" : "Save Settings"}
        </Button>
        <Button variant="outline" onClick={handleExport} disabled={exporting}>
          {exporting ? "Exporting…" : "Export YAML"}
        </Button>
        {saved && <p className="text-sm text-green-600">Saved!</p>}
      </div>
    </div>
  );
}
