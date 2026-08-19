import { renderHook, waitFor } from "@testing-library/react";
import { useUnreadNotifications } from "./useUnreadNotifications";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    getNotifications: jest.fn(),
  },
}));

describe("useUnreadNotifications", () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
    jest.useFakeTimers({ legacyFakeTimers: false });
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("does not fetch when no token is stored", async () => {
    const { result } = renderHook(() => useUnreadNotifications());

    await waitFor(() => expect(result.current.unreadCount).toBe(0));
    expect(api.getNotifications).not.toHaveBeenCalled();
  });

  it("uses the total from the response", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getNotifications as jest.Mock).mockResolvedValue({ total: 2 });

    const { result } = renderHook(() => useUnreadNotifications());

    await waitFor(() => expect(result.current.unreadCount).toBe(2));
  });

  it("exposes a refetch function that fetches immediately", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getNotifications as jest.Mock).mockResolvedValue({ total: 0 });

    const { result } = renderHook(() => useUnreadNotifications());
    await waitFor(() => expect(api.getNotifications).toHaveBeenCalledTimes(1));

    (api.getNotifications as jest.Mock).mockResolvedValue({ total: 5 });
    result.current.refetch();

    await waitFor(() => expect(result.current.unreadCount).toBe(5));
  });

  it("polls again after the interval elapses", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getNotifications as jest.Mock).mockResolvedValue({ total: 0 });

    renderHook(() => useUnreadNotifications());
    await waitFor(() => expect(api.getNotifications).toHaveBeenCalledTimes(1));

    jest.advanceTimersByTime(30_000);
    await waitFor(() => expect(api.getNotifications).toHaveBeenCalledTimes(2));
  });

  it("clears the polling interval on unmount", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getNotifications as jest.Mock).mockResolvedValue({ total: 0 });

    const { unmount } = renderHook(() => useUnreadNotifications());
    await waitFor(() => expect(api.getNotifications).toHaveBeenCalledTimes(1));

    unmount();
    jest.advanceTimersByTime(60_000);

    expect(api.getNotifications).toHaveBeenCalledTimes(1);
  });
});
