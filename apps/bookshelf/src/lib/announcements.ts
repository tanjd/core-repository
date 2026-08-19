import type { AnnouncementType } from "@/lib/types";

export const announcementTypeLabel: Record<AnnouncementType, string> = {
  info: "Info",
  new_feature: "New feature",
  known_issue: "Known issue",
};

export const announcementTypeVariant: Record<
  AnnouncementType,
  "outline" | "success" | "destructive"
> = {
  info: "outline",
  new_feature: "success",
  known_issue: "destructive",
};
