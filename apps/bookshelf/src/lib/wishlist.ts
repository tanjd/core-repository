import type { WishlistStatus } from "@/lib/types";

export const wishlistStatusLabel: Record<WishlistStatus, string> = {
  open: "Open",
  fulfilled: "Fulfilled",
  cancelled: "Cancelled",
};

export const wishlistStatusVariant: Record<
  WishlistStatus,
  "secondary" | "success" | "outline"
> = {
  open: "secondary",
  fulfilled: "success",
  cancelled: "outline",
};
