"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { AppSetting } from "@/lib/api";
import { Button } from "@/components/ui/button";

const SETTING_LABELS: Record<
  string,
  { label: string; description: string; type: "bool" | "number" | "text" }
> = {
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
  max_copies_per_user: {
    label: "Max Copies Per User",
    description:
      "Maximum number of book copies a user can share (0 = unlimited)",
    type: "number",
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
  require_email_confirmation_on_change: {
    label: "Confirm Email Changes via OTP",
    description:
      "Require users to verify ownership of a new email address (via a code sent to it) before it replaces their current one. Recommended to stay OFF until SMTP delivery is confirmed reliable in this deployment.",
    type: "bool",
  },
};

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

  if (loading)
    return <p className="text-muted-foreground">Loading settings…</p>;

  return (
    <div>
      <div className="space-y-6">
        {settings.map((setting) => {
          const meta = SETTING_LABELS[setting.key];
          if (!meta) return null;
          const type = meta.type;
          return (
            <div
              key={setting.key}
              className="flex items-start justify-between gap-4"
            >
              <div>
                <p className="font-medium text-sm">{meta.label}</p>
                {meta.description && (
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {meta.description}
                  </p>
                )}
              </div>
              <div className="shrink-0">
                {type === "bool" ? (
                  <button
                    onClick={() =>
                      setValues((prev) => ({
                        ...prev,
                        [setting.key]:
                          prev[setting.key] === "true" ? "false" : "true",
                      }))
                    }
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                      values[setting.key] === "true" ? "bg-primary" : "bg-muted"
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                        values[setting.key] === "true"
                          ? "translate-x-6"
                          : "translate-x-1"
                      }`}
                    />
                  </button>
                ) : (
                  <input
                    type="number"
                    value={values[setting.key] ?? ""}
                    onChange={(e) =>
                      setValues((prev) => ({
                        ...prev,
                        [setting.key]: e.target.value,
                      }))
                    }
                    className="w-20 rounded-md border px-2 py-1 text-sm text-right"
                    min={0}
                  />
                )}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mt-8 flex items-center gap-3">
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
