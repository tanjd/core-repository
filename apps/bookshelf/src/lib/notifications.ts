import { api } from "@/lib/api";
import type { Notification } from "@/lib/types";

export const notificationTypeLabel: Record<Notification["type"], string> = {
  request_received: "New borrow request",
  request_accepted: "Request accepted",
  request_rejected: "Request declined",
  marked_loaned: "Book loaned out",
  marked_returned: "Book returned",
  waitlist_available: "Copy now available",
  copy_transferred_in: "Copy transferred to you",
  copy_transferred_out: "Copy transfer sent",
  wishlist_fulfilled: "A book you wanted is available",
};

// Resolves where a click on a notification should navigate, mirroring the
// per-type destinations the loan-request/waitlist/copy-transfer flows land
// users on. Returns null when there's nothing to navigate to.
export async function notificationDestination(
  n: Notification,
): Promise<string | null> {
  if (n.type === "wishlist_fulfilled") {
    if (!n.wishlist_request_id) return null;
    try {
      const req = await api.getWishlistRequest(n.wishlist_request_id);
      return req.fulfilled_book_id
        ? `/catalog/${req.fulfilled_book_id}`
        : "/wishlist";
    } catch {
      return "/wishlist";
    }
  }

  if (!n.loan_request_id) return null;

  if (n.type === "request_received") {
    try {
      const lr = await api.getLoanRequest(n.loan_request_id);
      return `/my-books/${lr.copy_id}/requests`;
    } catch {
      return "/my-books";
    }
  }
  if (n.type === "waitlist_available") return "/catalog";
  if (n.type === "copy_transferred_in") return "/my-books";
  return "/my-requests";
}
