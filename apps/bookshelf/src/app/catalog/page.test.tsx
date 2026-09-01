import { act } from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import CatalogPage from "./page";
import { api } from "@/lib/api";
import type { Book, PaginatedResult } from "@/lib/types";

jest.mock("@/lib/api", () => ({
  api: {
    getBooks: jest.fn(),
  },
}));

jest.mock("@/hooks/useOwnedBookIds", () => ({
  useOwnedBookIds: () => new Set<number>(),
}));

jest.mock("@/components/BookshelfRow", () => ({
  BookshelfRow: () => null,
}));

const replace = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/catalog",
}));

function book(id: number, title: string): Book {
  return {
    id,
    title,
    author: "",
    isbn: "",
    ol_key: "",
    cover_url: "",
    description: "",
  };
}

function page(
  items: Book[],
  p: number,
  totalPages: number,
): PaginatedResult<Book> {
  return {
    items,
    total: items.length,
    page: p,
    page_size: 20,
    total_pages: totalPages,
  };
}

describe("CatalogPage fetch race guard", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers({ legacyFakeTimers: false });
    window.scrollTo = jest.fn();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("keeps the response for the most recently started fetch even if an older one resolves later", async () => {
    const resolvers: Array<(v: PaginatedResult<Book>) => void> = [];
    (api.getBooks as jest.Mock).mockImplementation(
      () => new Promise((resolve) => resolvers.push(resolve)),
    );

    render(<CatalogPage />);

    // Initial load (call #0) — resolve immediately with page 1.
    await act(async () => {
      resolvers[0](page([book(1, "Alpha")], 1, 3));
    });
    expect(screen.getAllByText("Alpha").length).toBeGreaterThan(0);

    // User types a search — debounced 300ms, then fires call #1 and is left
    // pending (simulating a slow response).
    fireEvent.change(screen.getByPlaceholderText("Search by title, author…"), {
      target: { value: "special" },
    });
    await act(async () => {
      await jest.advanceTimersByTimeAsync(300);
    });
    expect(api.getBooks).toHaveBeenCalledTimes(2);

    // Before that search resolves, the user clicks to page 2 — an immediate,
    // undebounced fetch (call #2) that resolves fast.
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(api.getBooks).toHaveBeenCalledTimes(3);
    await act(async () => {
      resolvers[2](page([book(2, "Beta")], 2, 3));
    });
    expect(screen.getAllByText("Beta").length).toBeGreaterThan(0);

    // The stale search fetch (call #1) finally resolves — it must NOT
    // clobber the newer pagination result now on screen.
    await act(async () => {
      resolvers[1](page([book(3, "Charlie")], 1, 1));
    });
    expect(screen.getAllByText("Beta").length).toBeGreaterThan(0);
    expect(screen.queryAllByText("Charlie").length).toBe(0);
  });
});
