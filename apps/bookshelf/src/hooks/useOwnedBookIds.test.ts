import { renderHook, waitFor } from "@testing-library/react";
import { useOwnedBookIds } from "./useOwnedBookIds";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    getMyOwnedBookIds: jest.fn(),
  },
}));

describe("useOwnedBookIds", () => {
  beforeEach(() => {
    localStorage.clear();
    jest.clearAllMocks();
  });

  it("does not fetch when no token is stored", () => {
    renderHook(() => useOwnedBookIds());

    expect(api.getMyOwnedBookIds).not.toHaveBeenCalled();
  });

  it("returns the set of book IDs the user owns copies of", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getMyOwnedBookIds as jest.Mock).mockResolvedValue({
      book_ids: [1, 2],
    });

    const { result } = renderHook(() => useOwnedBookIds());

    await waitFor(() => expect(result.current.size).toBe(2));
    expect(result.current).toEqual(new Set([1, 2]));
  });

  it("silently ignores a failed request", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (api.getMyOwnedBookIds as jest.Mock).mockRejectedValue(
      new Error("network"),
    );

    const { result } = renderHook(() => useOwnedBookIds());

    await waitFor(() => expect(api.getMyOwnedBookIds).toHaveBeenCalled());
    expect(result.current).toEqual(new Set());
  });
});
