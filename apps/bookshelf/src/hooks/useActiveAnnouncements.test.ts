import { act, renderHook, waitFor } from "@testing-library/react";
import { useActiveAnnouncements } from "./useActiveAnnouncements";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    getActiveAnnouncements: jest.fn(),
  },
}));

const ANNOUNCEMENTS = [
  {
    id: 2,
    title: "B",
    body: "b",
    type: "info" as const,
    active: true,
    created_at: "",
  },
  {
    id: 1,
    title: "A",
    body: "a",
    type: "info" as const,
    active: true,
    created_at: "",
  },
];

describe("useActiveAnnouncements", () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  it("does not fetch when no token is stored", async () => {
    const { result } = renderHook(() => useActiveAnnouncements());

    await waitFor(() => expect(result.current.announcement).toBeNull());
    expect(api.getActiveAnnouncements).not.toHaveBeenCalled();
  });

  it("fetches once on mount, not on an interval, and surfaces only the newest one", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getActiveAnnouncements as jest.Mock).mockResolvedValue(ANNOUNCEMENTS);

    const { result } = renderHook(() => useActiveAnnouncements());

    await waitFor(() => expect(result.current.announcement?.id).toBe(2));
    expect(api.getActiveAnnouncements).toHaveBeenCalledTimes(1);
  });

  it("clears the announcement once dismissed and persists the dismissal", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getActiveAnnouncements as jest.Mock).mockResolvedValue(ANNOUNCEMENTS);

    const { result } = renderHook(() => useActiveAnnouncements());
    await waitFor(() => expect(result.current.announcement?.id).toBe(2));

    act(() => result.current.dismiss(2));

    await waitFor(() => expect(result.current.announcement).toBeNull());
    expect(localStorage.getItem("bookshelf_dismissed_announcement_id")).toBe(
      "2",
    );
  });

  it("a dismissed announcement stays hidden on a fresh mount", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    localStorage.setItem("bookshelf_dismissed_announcement_id", "2");
    (api.getActiveAnnouncements as jest.Mock).mockResolvedValue(ANNOUNCEMENTS);

    const { result } = renderHook(() => useActiveAnnouncements());

    await waitFor(() => expect(api.getActiveAnnouncements).toHaveBeenCalled());
    expect(result.current.announcement).toBeNull();
  });

  it("shows a newer announcement again even if a previous one was dismissed", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    localStorage.setItem("bookshelf_dismissed_announcement_id", "1");
    (api.getActiveAnnouncements as jest.Mock).mockResolvedValue(ANNOUNCEMENTS);

    const { result } = renderHook(() => useActiveAnnouncements());

    await waitFor(() => expect(result.current.announcement?.id).toBe(2));
  });
});
