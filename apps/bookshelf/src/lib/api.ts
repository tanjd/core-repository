import type {
  User,
  Book,
  Copy,
  LoanRequest,
  Notification,
  AuthResponse,
  AppSetting,
  BookMetadataResult,
  MetadataProviderStatus,
  WaitlistStatus,
  PaginatedResult,
  JobStatus,
  VerificationStatus,
  DashboardStats,
} from "./types";

export type {
  User,
  Book,
  Copy,
  LoanRequest,
  Notification,
  AuthResponse,
  AppSetting,
  BookMetadataResult,
  MetadataProviderStatus,
  WaitlistStatus,
  PaginatedResult,
  VerificationStatus,
  DashboardStats,
};

/** Returns an error message if the password does not meet complexity requirements, or null if valid. */
export function validatePassword(password: string): string | null {
  if (password.length < 8) return "Password must be at least 8 characters";
  if (!/[A-Z]/.test(password))
    return "Password must contain at least one uppercase letter";
  if (!/[a-z]/.test(password))
    return "Password must contain at least one lowercase letter";
  if (!/[0-9]/.test(password))
    return "Password must contain at least one number";
  return null;
}

const BASE = "/api";

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("bookshelf_token");
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
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error ?? "Request failed");
  }
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

export const api = {
  // Auth
  setupStatus: () => request<{ needs_setup: boolean }>("/auth/setup-status"),
  setup: (data: { name: string; email: string; password: string }) =>
    request<AuthResponse>("/auth/setup", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  register: (data: { name: string; email: string; password: string }) =>
    request<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  login: (data: { email: string; password: string }) =>
    request<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  me: () => request<User>("/auth/me"),
  updateMe: (data: {
    name?: string;
    phone?: string;
    email?: string;
    google_books_api_key?: string;
  }) =>
    request<User>("/auth/me", { method: "PATCH", body: JSON.stringify(data) }),
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
    request<void>("/auth/send-otp", {
      method: "POST",
      body: JSON.stringify({}),
    }),
  verifyOTP: (code: string) =>
    request<User>("/auth/verify-otp", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),
  confirmEmailChange: (code: string) =>
    request<User>("/auth/confirm-email-change", {
      method: "POST",
      body: JSON.stringify({ code }),
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

  // Waitlist
  getWaitlistStatus: (copyId: number) =>
    request<WaitlistStatus>(`/copies/${copyId}/waitlist`),
  joinWaitlist: (copyId: number) =>
    request<void>(`/copies/${copyId}/waitlist`, { method: "POST" }),
  leaveWaitlist: (copyId: number) =>
    request<void>(`/copies/${copyId}/waitlist`, { method: "DELETE" }),

  // Loan requests
  getMyLoanRequests: (params?: { page?: number; page_size?: number }) => {
    const p: Record<string, string> = {};
    if (params?.page) p.page = String(params.page);
    if (params?.page_size) p.page_size = String(params.page_size);
    const qs = new URLSearchParams(p).toString();
    return request<PaginatedResult<LoanRequest>>(
      `/loan-requests/mine${qs ? "?" + qs : ""}`,
    );
  },
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
    data: { role?: "user" | "admin"; suspended?: boolean },
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

  // Metadata provider status
  adminGetMetadataStatus: () =>
    request<MetadataProviderStatus[]>("/admin/metadata/status"),
};
