import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import NotificationsPage from "./page";
import { api } from "@/lib/api";
import type { Notification, PaginatedResult } from "@/lib/types";

jest.mock("@/lib/api", () => ({
  api: {
    getNotifications: jest.fn(),
    markNotificationRead: jest.fn(),
    markAllRead: jest.fn(),
  },
}));

const push = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

function notification(overrides: Partial<Notification> = {}): Notification {
  return {
    id: 1,
    recipient_id: 1,
    type: "waitlist_available",
    read: false,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  } as Notification;
}

function page(items: Notification[]): PaginatedResult<Notification> {
  return { items, total: items.length, page: 1, page_size: 20, total_pages: 1 };
}

describe("NotificationsPage unread count", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.setItem("bookshelf_token", "test-token");
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("shows the hook's total unread count, not just unread items on the current page", async () => {
    // The visible page has only 1 unread item, but the account has 5 unread
    // overall (e.g. more sit on page 2+) — the badge must reflect the real
    // total, not `notifications.filter(n => !n.read).length`.
    (api.getNotifications as jest.Mock).mockImplementation(({ unread }) => {
      if (unread) return Promise.resolve({ ...page([]), total: 5 });
      return Promise.resolve(
        page([
          notification({ id: 1, read: false }),
          notification({ id: 2, read: true }),
        ]),
      );
    });

    render(<NotificationsPage />);

    expect(await screen.findByText("5 unread")).toBeInTheDocument();
  });

  it("does not show an unread badge or Mark all read button when the account has zero unread", async () => {
    (api.getNotifications as jest.Mock).mockImplementation(({ unread }) => {
      if (unread) return Promise.resolve({ ...page([]), total: 0 });
      return Promise.resolve(page([notification({ read: true })]));
    });

    render(<NotificationsPage />);

    await screen.findByText("Copy now available");
    expect(screen.queryByText(/unread/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /mark all read/i }),
    ).not.toBeInTheDocument();
  });
});

describe("NotificationsPage keeps the nav bell in sync", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.setItem("bookshelf_token", "test-token");
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("re-fetches the unread total after marking a single notification read", async () => {
    (api.getNotifications as jest.Mock).mockImplementation(({ unread }) => {
      if (unread) return Promise.resolve({ ...page([]), total: 3 });
      return Promise.resolve(page([notification({ id: 1, read: false })]));
    });
    (api.markNotificationRead as jest.Mock).mockResolvedValue({});

    const user = userEvent.setup();
    render(<NotificationsPage />);

    await screen.findByText("3 unread");
    const unreadCallsBefore = (
      api.getNotifications as jest.Mock
    ).mock.calls.filter((c) => c[0]?.unread).length;

    await user.click(
      screen.getByRole("button", { name: /copy now available/i }),
    );

    await waitFor(() => {
      const unreadCallsAfter = (
        api.getNotifications as jest.Mock
      ).mock.calls.filter((c) => c[0]?.unread).length;
      expect(unreadCallsAfter).toBeGreaterThan(unreadCallsBefore);
    });
  });

  it("re-fetches the unread total after Mark all read", async () => {
    (api.getNotifications as jest.Mock).mockImplementation(({ unread }) => {
      if (unread) return Promise.resolve({ ...page([]), total: 2 });
      return Promise.resolve(
        page([
          notification({ id: 1, read: false }),
          notification({ id: 2, read: false }),
        ]),
      );
    });
    (api.markAllRead as jest.Mock).mockResolvedValue({});

    const user = userEvent.setup();
    render(<NotificationsPage />);

    await screen.findByText("2 unread");
    const unreadCallsBefore = (
      api.getNotifications as jest.Mock
    ).mock.calls.filter((c) => c[0]?.unread).length;

    await user.click(screen.getByRole("button", { name: /mark all read/i }));

    await waitFor(() => {
      expect(api.markAllRead).toHaveBeenCalled();
      const unreadCallsAfter = (
        api.getNotifications as jest.Mock
      ).mock.calls.filter((c) => c[0]?.unread).length;
      expect(unreadCallsAfter).toBeGreaterThan(unreadCallsBefore);
    });
  });
});
