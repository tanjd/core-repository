import type {
  User,
  Book,
  Copy,
  LoanRequest,
  Notification,
  Announcement,
  AnnouncementType,
  AuthResponse,
  AppSetting,
  BookMetadataResult,
  MetadataProviderStatus,
  WaitlistStatus,
  PaginatedResult,
  JobStatus,
  BackupInfo,
  VerificationStatus,
  DashboardStats,
  WishlistRequest,
} from "./types";

export type {
  User,
  Book,
  Copy,
  LoanRequest,
  Notification,
  Announcement,
  AnnouncementType,
  AuthResponse,
  AppSetting,
  BookMetadataResult,
  MetadataProviderStatus,
  WaitlistStatus,
  PaginatedResult,
  VerificationStatus,
  DashboardStats,
  WishlistRequest,
};

// Mirrors internal/handlers/auth.go's minPasswordLength/maxPasswordLength —
// there's no shared validation module across the Go/TS boundary, so these
// (and COMMON_PASSWORDS below) are kept in sync by hand.
export const MIN_PASSWORD_LENGTH = 12;
export const MAX_PASSWORD_LENGTH = 72;

// Mirrors internal/handlers/auth.go's commonPasswords denylist.
const COMMON_PASSWORDS = new Set([
  "123456",
  "123456789",
  "12345678",
  "1234567890",
  "1234567",
  "1234",
  "12345",
  "111111",
  "000000",
  "123123",
  "password",
  "password1",
  "password123",
  "passw0rd",
  "iloveyou",
  "qwerty",
  "qwerty123",
  "qwertyuiop",
  "qazwsx",
  "azerty",
  "abc123",
  "letmein",
  "letmein1",
  "welcome",
  "welcome1",
  "monkey",
  "dragon",
  "master",
  "shadow",
  "superman",
  "batman",
  "football",
  "baseball",
  "starwars",
  "sunshine",
  "princess",
  "flower",
  "freedom",
  "whatever",
  "trustno1",
  "admin",
  "admin123",
  "administrator",
  "root",
  "toor",
  "guest",
  "guest123",
  "test",
  "test123",
  "temp123",
  "changeme",
  "changeme123",
  "hunter2",
  "michael",
  "ashley",
  "bailey",
  "jennifer",
  "jordan",
  "michelle",
  "mustang",
  "ninja",
  "121212",
  "123321",
  "654321",
  "1q2w3e4r",
  "zaq1zaq1",
  "1qaz2wsx",
  "aa123456",
  "google",
  "bookshelf",
  "bookshelf1",
  "bookshelf123",
]);

/**
 * Returns an error message if the password does not meet complexity
 * requirements, or null if valid. `disallowed` is a set of
 * user-identifying strings (e.g. name, email local part) the password must
 * not contain — pass what's available on the form; omit where there's
 * nothing to check against yet (e.g. registration's name field is only
 * checked once both fields exist).
 */
export function validatePassword(
  password: string,
  disallowed: string[] = [],
): string | null {
  if (password.length < MIN_PASSWORD_LENGTH)
    return `Password must be at least ${MIN_PASSWORD_LENGTH} characters`;
  if (password.length > MAX_PASSWORD_LENGTH)
    return `Password must be at most ${MAX_PASSWORD_LENGTH} characters`;
  if (!/[A-Z]/.test(password))
    return "Password must contain at least one uppercase letter";
  if (!/[a-z]/.test(password))
    return "Password must contain at least one lowercase letter";
  if (!/[0-9]/.test(password))
    return "Password must contain at least one number";

  const lower = password.toLowerCase();
  if (COMMON_PASSWORDS.has(lower))
    return "This password is too common — please choose a stronger one";
  for (const d of disallowed) {
    const needle = d.trim().toLowerCase();
    if (needle.length >= 3 && lower.includes(needle))
      return "Password must not contain your name or email";
  }
  return null;
}

/** Returns the portion of email before "@", for use as a validatePassword disallowed entry. */
export function emailLocalPart(email: string): string {
  const at = email.indexOf("@");
  return at > 0 ? email.slice(0, at) : email;
}

export type PasswordStrengthScore = 0 | 1 | 2 | 3 | 4;

export interface PasswordStrength {
  score: PasswordStrengthScore;
  label: "Very weak" | "Weak" | "Fair" | "Good" | "Strong";
}

const STRENGTH_LABELS = [
  "Very weak",
  "Weak",
  "Fair",
  "Good",
  "Strong",
] as const;

/**
 * A lightweight, dependency-free password strength estimate for live UI
 * feedback (not a substitute for validatePassword's hard requirements).
 * Rewards length and character variety, penalizes common passwords and
 * repeated/sequential runs (e.g. "aaaa", "1234", "abcd").
 */
export function scorePasswordStrength(password: string): PasswordStrength {
  if (!password) return { score: 0, label: STRENGTH_LABELS[0] };

  const lower = password.toLowerCase();
  if (COMMON_PASSWORDS.has(lower))
    return { score: 0, label: STRENGTH_LABELS[0] };

  let rawScore = 0;
  if (password.length >= MIN_PASSWORD_LENGTH) rawScore++;
  if (password.length >= 16) rawScore++;
  if (password.length >= 20) rawScore++;

  const varietyCount = [/[a-z]/, /[A-Z]/, /[0-9]/, /[^a-zA-Z0-9]/].filter(
    (re) => re.test(password),
  ).length;
  if (varietyCount >= 3) rawScore++;

  if (/(.)\1\1/.test(password)) rawScore--; // 3+ repeated chars, e.g. "aaa"
  if (hasSequentialRun(password, 4)) rawScore--; // e.g. "abcd", "1234", "4321"

  const score = Math.max(0, Math.min(4, rawScore)) as PasswordStrengthScore;
  return { score, label: STRENGTH_LABELS[score] };
}

/** True if password contains `runLength` consecutive ascending or descending character codes. */
function hasSequentialRun(password: string, runLength: number): boolean {
  let ascending = 1;
  let descending = 1;
  for (let i = 1; i < password.length; i++) {
    const delta = password.charCodeAt(i) - password.charCodeAt(i - 1);
    ascending = delta === 1 ? ascending + 1 : 1;
    descending = delta === -1 ? descending + 1 : 1;
    if (ascending >= runLength || descending >= runLength) return true;
  }
  return false;
}

const BASE = "/api";

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("bookshelf_token");
}

/**
 * Fetches url with the auth header and saves the response via the browser as
 * filename — for binary/attachment responses, which can't go through
 * request<T>(), which always calls res.json().
 */
async function downloadAuthed(url: string, filename: string): Promise<void> {
  const token = getToken();
  const res = await fetch(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ detail: res.statusText }));
    throw new Error(err.detail ?? "Download failed");
  }
  const blob = await res.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = objectUrl;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(objectUrl);
}

/** Downloads a backup archive, same UX as handleExport() in the settings page. */
export async function downloadBackup(filename: string): Promise<void> {
  return downloadAuthed(
    `${BASE}/admin/backups/${encodeURIComponent(filename)}/download`,
    filename,
  );
}

export type MyCopiesExportFormat = "json" | "yaml" | "csv";

/** Downloads the caller's owned copies in the given export format. */
export async function downloadMyCopiesExport(
  format: MyCopiesExportFormat,
): Promise<void> {
  return downloadAuthed(
    `${BASE}/copies/mine/export?format=${format}`,
    `my-books.${format}`,
  );
}

export type ImportRowAction = "create_book" | "match_existing_book" | "skipped";

export interface ImportRowResult {
  row: number;
  title: string;
  action: ImportRowAction;
  reason?: string;
}

export interface ImportSummary {
  books_created: number;
  books_matched: number;
  copies_created: number;
  skipped: number;
}

export interface ImportResult {
  summary: ImportSummary;
  rows: ImportRowResult[];
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken();
  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options?.headers ?? {}),
  };
  const res = await fetch(`${BASE}${path}`, { ...options, headers });
  if (!res.ok) {
    // huma's ErrorModel puts the message in `detail` (see e.g. auth.go's
    // huma.Error400BadRequest calls), not `error` — this previously always
    // fell through to the generic fallback below for every API error.
    const err = await res.json().catch(() => ({ detail: res.statusText }));
    throw new Error(err.detail ?? "Request failed");
  }
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

export const api = {
  // Auth
  setupStatus: () => request<{ needs_setup: boolean }>("/auth/setup-status"),
  registrationRequirements: () =>
    request<{ require_phone: boolean }>("/auth/registration-requirements"),
  setup: (data: { name: string; email: string; password: string }) =>
    request<AuthResponse>("/auth/setup", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  register: (data: {
    name: string;
    email: string;
    password: string;
    phone?: string;
    email_verification_token: string;
    phone_verification_token?: string;
  }) =>
    request<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  sendRegisterEmailOTP: (email: string) =>
    request<{ debug_code?: string }>("/auth/register/send-email-otp", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
  verifyRegisterEmailOTP: (
    data: { email: string; code: string } | { token: string },
  ) =>
    request<{ verification_token: string; email: string }>(
      "/auth/register/verify-email-otp",
      {
        method: "POST",
        body: JSON.stringify(data),
      },
    ),
  sendRegisterPhoneOTP: (phone: string) =>
    request<{ mock_code: string }>("/auth/register/send-phone-otp", {
      method: "POST",
      body: JSON.stringify({ phone }),
    }),
  verifyRegisterPhoneOTP: (phone: string, code: string) =>
    request<{ verification_token: string }>("/auth/register/verify-phone-otp", {
      method: "POST",
      body: JSON.stringify({ phone, code }),
    }),
  login: (data: { email: string; password: string }) =>
    request<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  forgotPassword: (email: string) =>
    request<{ debug_code?: string }>("/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
  resetPassword: (
    data: ({ email: string; code: string } | { token: string }) & {
      new_password: string;
      confirm_password: string;
    },
  ) =>
    request<void>("/auth/reset-password", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  me: () => request<User>("/auth/me"),
  updateMe: (data: {
    name?: string;
    phone?: string;
    email?: string;
    google_books_api_key?: string;
    email_notifications_enabled?: boolean;
    telegram_username?: string;
    whatsapp_username?: string;
  }) =>
    request<User & { pending_email_debug_code?: string }>("/auth/me", {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  changePassword: (data: {
    current_password: string;
    new_password: string;
    confirm_password: string;
  }) =>
    request<void>("/auth/me/password", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  testGoogleBooksKey: (key?: string) =>
    request<{ ok: boolean; message?: string }>(
      "/auth/me/google-books-key/test",
      { method: "POST", body: JSON.stringify({ key: key ?? "" }) },
    ),
  sendOTP: () =>
    request<{ debug_code?: string }>("/auth/send-otp", {
      method: "POST",
      body: JSON.stringify({}),
    }),
  verifyOTP: (data: { code: string } | { token: string }) =>
    request<User>("/auth/verify-otp", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  confirmEmailChange: (data: { code: string } | { token: string }) =>
    request<User>("/auth/confirm-email-change", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  myVerificationStatus: () =>
    request<VerificationStatus>("/auth/me/verification-status"),

  // Books
  getBooks: (params?: {
    q?: string;
    ol_key?: string;
    sort?: string;
    available_only?: boolean;
    page?: number;
    page_size?: number;
  }) => {
    const p: Record<string, string> = {};
    if (params?.q) p.q = params.q;
    if (params?.ol_key) p.ol_key = params.ol_key;
    if (params?.sort) p.sort = params.sort;
    if (params?.available_only) p.available_only = "true";
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<Book>>(`/books${qs ? "?" + qs : ""}`);
  },
  getRecentBooks: (limit?: number) =>
    request<Book[]>(`/books/recent${limit ? "?limit=" + limit : ""}`),
  getBook: (id: number) => request<Book>(`/books/${id}`),
  createBook: (data: Partial<Book>) =>
    request<Book>("/books", { method: "POST", body: JSON.stringify(data) }),

  // Metadata search (proxied through backend)
  searchMetadata: (q: string) =>
    request<BookMetadataResult[]>(
      `/books/metadata/search?q=${encodeURIComponent(q)}`,
    ),
  getOLDescription: (olKey: string) =>
    request<{ description: string }>(
      `/books/metadata/ol-description?ol_key=${encodeURIComponent(olKey)}`,
    ),

  // Copies
  getMyCopies: () => request<Copy[]>("/copies/mine"),
  getMyOwnedBookIds: () =>
    request<{ book_ids: number[] }>("/copies/mine/book-ids"),
  createCopy: (data: {
    book_id: number;
    condition: string;
    notes?: string;
    auto_approve?: boolean;
    return_date_required?: boolean;
    hide_owner?: boolean;
  }) =>
    request<Copy>("/copies", { method: "POST", body: JSON.stringify(data) }),
  updateCopy: (
    id: number,
    data: {
      condition?: string;
      notes?: string;
      status?: string;
      auto_approve?: boolean;
      return_date_required?: boolean;
      hide_owner?: boolean;
    },
  ) =>
    request<Copy>(`/copies/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  deleteCopy: (id: number) =>
    request<void>(`/copies/${id}`, { method: "DELETE" }),
  transferCopy: (id: number, email: string) =>
    request<Copy>(`/copies/${id}/transfer`, {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
  previewImportBooks: (format: MyCopiesExportFormat, content: string) =>
    request<ImportResult>("/copies/mine/import/preview", {
      method: "POST",
      body: JSON.stringify({ format, content }),
    }),
  importBooks: (format: MyCopiesExportFormat, content: string) =>
    request<ImportResult>("/copies/mine/import", {
      method: "POST",
      body: JSON.stringify({ format, content }),
    }),

  // Waitlist
  getWaitlistStatus: (copyId: number) =>
    request<WaitlistStatus>(`/copies/${copyId}/waitlist`),
  joinWaitlist: (copyId: number) =>
    request<void>(`/copies/${copyId}/waitlist`, { method: "POST" }),
  leaveWaitlist: (copyId: number) =>
    request<void>(`/copies/${copyId}/waitlist`, { method: "DELETE" }),

  // Loan requests
  getMyLoanRequests: (params?: {
    page?: number;
    page_size?: number;
    view?: "current" | "history";
  }) => {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    if (params?.view) p.view = params.view;
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<LoanRequest>>(
      `/loan-requests/mine${qs ? "?" + qs : ""}`,
    );
  },
  getMyActiveLoans: () =>
    request<{ items: LoanRequest[] }>("/loan-requests/mine/active"),
  getLoanRequestsByCopy: (copyId: number) =>
    request<LoanRequest[]>(`/loan-requests?copy_id=${copyId}`),
  createLoanRequest: (data: {
    copy_id: number;
    message?: string;
    expected_return_date?: string;
  }) =>
    request<LoanRequest>("/loan-requests", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  getLoanRequest: (id: number) => request<LoanRequest>(`/loan-requests/${id}`),
  updateLoanRequest: (
    id: number,
    data: { status: string; new_condition?: string },
  ) =>
    request<LoanRequest>(`/loan-requests/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  updateExpectedReturnDate: (id: number, expectedReturnDate: string) =>
    request<LoanRequest>(`/loan-requests/${id}/expected-return-date`, {
      method: "PATCH",
      body: JSON.stringify({ expected_return_date: expectedReturnDate }),
    }),

  // Notifications
  getNotifications: (params?: {
    unread?: boolean;
    page?: number;
    page_size?: number;
  }) => {
    const p: Record<string, string> = {};
    if (params?.unread) p.unread = "true";
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<Notification>>(
      `/notifications${qs ? "?" + qs : ""}`,
    );
  },
  markNotificationRead: (id: number) =>
    request<void>(`/notifications/${id}/read`, { method: "PATCH" }),
  markAllRead: () =>
    request<void>("/notifications/read-all", { method: "PATCH" }),

  // Announcements
  getActiveAnnouncements: () => request<Announcement[]>("/announcements"),
  adminListAnnouncements: (params?: { page?: number; page_size?: number }) => {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<Announcement>>(
      `/admin/announcements${qs ? "?" + qs : ""}`,
    );
  },
  adminCreateAnnouncement: (data: {
    title: string;
    body: string;
    type: AnnouncementType;
    active?: boolean;
  }) =>
    request<Announcement>("/admin/announcements", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  adminUpdateAnnouncement: (
    id: number,
    data: {
      title?: string;
      body?: string;
      type?: AnnouncementType;
      active?: boolean;
    },
  ) =>
    request<Announcement>(`/admin/announcements/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  adminDeleteAnnouncement: (id: number) =>
    request<void>(`/admin/announcements/${id}`, { method: "DELETE" }),

  // Wishlist
  getWishlistRequests: (params?: {
    q?: string;
    page?: number;
    page_size?: number;
  }) => {
    const p: Record<string, string> = {};
    if (params?.q) p.q = params.q;
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<WishlistRequest>>(
      `/wishlist${qs ? "?" + qs : ""}`,
    );
  },
  getMyWishlistRequests: () => request<WishlistRequest[]>("/wishlist/mine"),
  getWishlistRequest: (id: number) =>
    request<WishlistRequest>(`/wishlist/${id}`),
  createWishlistRequest: (data: {
    title: string;
    author: string;
    isbn?: string;
    ol_key?: string;
    google_books_id?: string;
    cover_url?: string;
    notes?: string;
    is_anonymous?: boolean;
  }) =>
    request<WishlistRequest>("/wishlist", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  checkWishlistRequest: (params: {
    isbn?: string;
    ol_key?: string;
    google_books_id?: string;
  }) => {
    const p: Record<string, string> = {};
    if (params.isbn) p.isbn = params.isbn;
    if (params.ol_key) p.ol_key = params.ol_key;
    if (params.google_books_id) p.google_books_id = params.google_books_id;
    const qs = new URLSearchParams(p).toString();
    return request<{ match: WishlistRequest | null }>(
      `/wishlist/check${qs ? "?" + qs : ""}`,
    );
  },
  cancelWishlistRequest: (id: number) =>
    request<void>(`/wishlist/${id}`, { method: "DELETE" }),
  fulfillWishlistRequest: (id: number, bookId: number) =>
    request<WishlistRequest>(`/wishlist/${id}/fulfill`, {
      method: "POST",
      body: JSON.stringify({ book_id: bookId }),
    }),

  // Admin
  adminGetDashboardStats: () => request<DashboardStats>("/admin/dashboard"),
  adminListUsers: (params?: { page?: number; page_size?: number }) => {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<User>>(`/admin/users${qs ? "?" + qs : ""}`);
  },
  adminUpdateUser: (
    id: number,
    data: {
      role?: "user" | "admin";
      suspended?: boolean;
      pending_approval?: boolean;
    },
  ) =>
    request<User>(`/admin/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  adminDeleteUser: (id: number) =>
    request<void>(`/admin/users/${id}`, { method: "DELETE" }),
  adminGetSettings: () => request<AppSetting[]>("/admin/settings"),
  adminUpdateSettings: (settings: { key: string; value: string }[]) =>
    request<AppSetting[]>("/admin/settings", {
      method: "PATCH",
      body: JSON.stringify(settings),
    }),
  adminExportSettings: () =>
    request<{ content: string }>("/admin/settings/export"),

  // Jobs
  adminGetJobs: () => request<JobStatus[]>("/admin/jobs"),
  adminRunJob: (job: string) =>
    request<void>(`/admin/jobs/${job}/run`, { method: "POST" }),

  // Backups
  adminListBackups: () => request<BackupInfo[]>("/admin/backups"),
  adminDeleteBackup: (filename: string) =>
    request<void>(`/admin/backups/${encodeURIComponent(filename)}`, {
      method: "DELETE",
    }),

  // Metadata provider status
  adminGetMetadataStatus: () =>
    request<MetadataProviderStatus[]>("/admin/metadata/status"),
};
