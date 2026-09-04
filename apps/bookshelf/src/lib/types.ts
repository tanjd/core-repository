export interface User {
  id: number;
  name: string;
  email: string;
  phone: string;
  verified: boolean;
  phone_verified: boolean;
  suspended: boolean;
  pending_approval: boolean;
  role: "user" | "admin";
  created_at: string;
  google_books_key_configured: boolean;
  pending_email?: string;
  email_notifications_enabled: boolean;
  monthly_digest_enabled: boolean;
  // telegram_linked/telegram_notifications_enabled: the bot-linked push
  // notification channel (see docs/telegram-bot-integration-spec.md) — not
  // to be confused with telegram_username below, a free-text contact field
  // shown to the other party in an accepted loan.
  telegram_linked: boolean;
  telegram_notifications_enabled: boolean;
  telegram_username?: string;
  whatsapp_username?: string;
  contact_note?: string;
  // invited_by_id/invited_by: which member's invite link this account
  // registered through, if any — see apps/bookshelf/docs/invite-code-spec.md.
  // invited_by is preloaded on GET /admin/users only.
  invited_by_id?: number;
  invited_by?: User;
}

// PublicContact is the redacted view of a user shown to the other party in a
// loan request — id/name always, contact fields only once the loan is
// accepted (see bookshelf-backend's safeUser/buildContactPair).
export interface PublicContact {
  id: number;
  name: string;
  email?: string;
  phone?: string;
  telegram_username?: string;
  whatsapp_username?: string;
  contact_note?: string;
}

export interface AppSetting {
  key: string;
  value: string;
  updated_at: string;
}

export type AnnouncementType = "info" | "new_feature" | "known_issue";

export interface Announcement {
  id: number;
  title: string;
  body: string;
  type: AnnouncementType;
  active: boolean;
  created_at: string;
}

export interface Book {
  id: number;
  title: string;
  author: string;
  isbn: string;
  ol_key: string;
  cover_url: string;
  description: string;
  publisher?: string;
  published_date?: string;
  page_count?: number;
  language?: string;
  google_books_id?: string;
  created_at?: string;
  copies?: Copy[];
  available_copies?: number;
  // borrow_count: completed loans (LoanRequest status accepted/returned)
  // against any copy of this book. waitlist_count: live waitlist depth
  // across every copy. Both are populated by the backend on GET /books
  // and GET /books/{id} — see
  // apps/bookshelf/docs/community-reading-activity-spec.md.
  borrow_count?: number;
  waitlist_count?: number;
  description_enriched?: boolean;
  // recommendation_count: how many current members have given this book a
  // "highly recommend this" thumbs-up. your_recommendation: whether the
  // current viewer has (always false for an anonymous viewer). Both are
  // populated by the backend on GET /books and GET /books/{id} — see
  // apps/bookshelf/docs/book-recommendations-spec.md.
  recommendation_count?: number;
  your_recommendation?: boolean;
}

// Recommendation is one entry in a book's recommender list — the wire shape
// returned by GET /books/{id}/recommendations, newest first. See
// apps/bookshelf/docs/book-recommendations-spec.md.
export interface Recommendation {
  recommender_name: string;
  created_at: string;
}

// InviteCode is one row in the admin invite-links table, returned by
// GET /admin/invite-codes. See apps/bookshelf/docs/invite-code-spec.md.
export interface InviteCode {
  id: number;
  code: string;
  inviter_id: number;
  inviter_name: string;
  created_at: string;
}

export interface Copy {
  id: number;
  book_id: number;
  owner_id: number;
  condition: "good" | "fair" | "worn";
  notes: string;
  status: "available" | "requested" | "loaned" | "unavailable";
  auto_approve?: boolean;
  hide_owner?: boolean;
  book?: Book;
  owner?: PublicContact;
}

export interface LoanRequest {
  id: number;
  copy_id: number;
  borrower_id: number;
  message: string;
  status: "pending" | "accepted" | "rejected" | "cancelled" | "returned";
  requested_at: string;
  responded_at?: string;
  loaned_at?: string;
  returned_at?: string;
  returned_by?: number;
  expected_return_date: string;
  expected_return_date_changed_by?: number;
  expected_return_date_changed_at?: string;
  copy?: Copy;
  borrower?: PublicContact;
}

export interface Notification {
  id: number;
  recipient_id: number;
  type:
    | "request_received"
    | "request_accepted"
    | "request_rejected"
    | "marked_loaned"
    | "marked_returned"
    | "return_undone"
    | "waitlist_available"
    | "copy_transferred_in"
    | "copy_transferred_out"
    | "wishlist_fulfilled"
    | "user_pending_approval"
    | "user_approved"
    | "expected_return_date_changed";
  loan_request_id?: number;
  wishlist_request_id?: number;
  pending_user_id?: number;
  read: boolean;
  created_at: string;
}

export type WishlistStatus = "open" | "fulfilled" | "cancelled";

export interface WishlistRequest {
  id: number;
  requester_id: number;
  title: string;
  author: string;
  isbn: string;
  ol_key: string;
  google_books_id: string;
  cover_url: string;
  notes: string;
  status: WishlistStatus;
  is_anonymous: boolean;
  fulfilled_book_id?: number;
  fulfilled_at?: string;
  created_at: string;
  requester?: { id: number; name: string };
  fulfilled_book?: Book;
}

export interface WaitlistEntry {
  id: number;
  copy_id: number;
  user_id: number;
  created_at: string;
  user?: { id: number; name: string };
}

export interface WaitlistStatus {
  count: number;
  on_waitlist: boolean;
}

export interface JobStatus {
  name: string;
  running: boolean;
  interval: string;
  last_run_at: string | null;
  next_run_at: string | null;
  last_result: string;
}

// TelegramBotStatus is whether apps/bookshelf-bot's own process is up —
// distinct from a member's own Telegram link (User.telegram_linked), and
// from whether push delivery works (needs only the backend's bot token).
export interface TelegramBotStatus {
  configured: boolean;
  online: boolean;
}

export interface BackupInfo {
  filename: string;
  size_bytes: number;
  created_at: string;
}

export interface PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface AuthResponse {
  token: string;
  user: User;
}

/**
 * Outcome of `POST /auth/register/verify-email-otp`, which is where an
 * account is actually created — both by typing the emailed code and by
 * clicking the emailed magic link. A `complete` result carries a live
 * session; `pending_approval` means the account exists but an admin has to
 * let it in first, so there's no token to store.
 */
export type RegistrationResult =
  | { status: "complete"; token: string; user: User }
  | { status: "pending_approval"; token?: never; user: User };

// Normalised metadata search result (from backend proxy)
export interface BookMetadataResult {
  source: "openlibrary" | "google_books" | "bookbrainz";
  title: string;
  author: string;
  isbn: string;
  cover_url: string;
  description: string;
  publisher: string;
  published_date: string;
  page_count: number;
  language: string;
  ol_key: string;
  google_books_id: string;
  bookbrainz_id?: string;
  enriched_fields?: string[];
  work_key?: string;
}

export interface VerificationFactor {
  key: "email" | "phone" | "min_books_shared";
  label: string;
  required: boolean;
  satisfied: boolean;
  target?: number;
  current?: number;
}

export interface VerificationStatus {
  eligible: boolean;
  factors: VerificationFactor[];
}

export interface MetadataProviderStatus {
  name: string;
  enabled: boolean;
  reachable: boolean;
  latency_ms: number;
  error?: string;
}

export interface BookBorrowStat {
  book_id: number;
  title: string;
  author: string;
  borrow_count: number;
}

export interface LenderStat {
  user_id: number;
  name: string;
  active_loans: number;
}

export interface ContributorStat {
  user_id: number;
  name: string;
  copy_count: number;
}

export interface DashboardStats {
  total_books: number;
  total_copies: number;
  available_copies: number;
  loaned_copies: number;
  total_users: number;
  signups_this_week: number;
  overdue_count: number;
  pending_approval_count: number;
  active_loans_count: number;
  completed_loans_count: number;
  most_borrowed_books: BookBorrowStat[];
  active_lenders: LenderStat[];
  top_contributors: ContributorStat[];
}
