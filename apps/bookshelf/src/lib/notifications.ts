import { api } from "@/lib/api";
import type { Notification } from "@/lib/types";

export const notificationTypeLabel: Record<Notification["type"], string> = {
  request_received: "New borrow request",
  request_accepted: "Request accepted",
  request_rejected: "Request declined",
  marked_loaned: "Book loaned out",
  marked_returned: "Book returned",
  return_undone: "Return undone",
  waitlist_available: "Copy now available",
  copy_transferred_in: "Copy transferred to you",
  copy_transferred_out: "Copy transfer sent",
  wishlist_fulfilled: "A book you wanted is available",
  user_pending_approval: "New user awaiting approval",
  user_approved: "Your account has been approved",
  expected_return_date_changed: "Return date changed",
};

// The currently logged-in user's id, as stashed in localStorage at login
// (see my-books/page.tsx's auth check for the same read pattern). Used to
// tell which of the two parties on a loan request the current user is.
function currentUserID(): number | null {
  if (typeof window === "undefined") return null;
  const stored = localStorage.getItem("bookshelf_user");
  if (!stored) return null;
  try {
    const user = JSON.parse(stored) as { id?: number };
    return user.id ?? null;
  } catch {
    return null;
  }
}

// Resolves where a click on a notification should navigate, mirroring the
// per-type destinations the loan-request/waitlist/copy-transfer flows land
// users on. Returns null when there's nothing to navigate to.
export async function notificationDestination(
  n: Notification,
): Promise<string | null> {
  if (n.type === "user_pending_approval") return "/admin/users";
  if (n.type === "user_approved") return null;

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
  if (
    n.type === "marked_returned" ||
    n.type === "expected_return_date_changed"
  ) {
    // Either party can trigger a return or a return-date change, so the
    // recipient here might be the owner or the borrower — route each to
    // their own view of the loan.
    const currentUserId = currentUserID();
    if (currentUserId !== null) {
      try {
        const lr = await api.getLoanRequest(n.loan_request_id);
        return currentUserId === lr.borrower_id
          ? "/my-requests"
          : `/my-books/${lr.copy_id}/requests`;
      } catch {
        // fall through to the borrower-oriented default below
      }
    }
  }
  if (n.type === "waitlist_available") return "/catalog";
  if (n.type === "copy_transferred_in") return "/my-books";
  return "/my-requests";
}
