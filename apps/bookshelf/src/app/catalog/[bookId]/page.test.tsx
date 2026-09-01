import { act } from "react";
import { render, screen } from "@testing-library/react";
import BookDetailPage from "./page";
import { api } from "@/lib/api";
import type { Book } from "@/lib/types";

jest.mock("@/lib/api", () => ({
  api: {
    getBook: jest.fn(),
    getRecommendations: jest.fn().mockResolvedValue([]),
  },
}));

// Mutable so a test can simulate the route's bookId changing (e.g. the user
// clicking through to a different book) while this same page component
// instance stays mounted — mirrors how Next.js app router reuses a page
// component across a dynamic-segment navigation. Prefixed with "mock" so
// Jest's module-factory hoisting allows referencing it from the factory.
let mockBookId = "1";
jest.mock("next/navigation", () => ({
  useParams: () => ({ bookId: mockBookId }),
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

function book(overrides: Partial<Book> = {}): Book {
  return {
    id: 1,
    title: "Dune",
    author: "Frank Herbert",
    isbn: "",
    ol_key: "",
    cover_url: "",
    description: "",
    copies: [],
    ...overrides,
  };
}

describe("BookDetailPage loadBook race guard", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.clear();
    mockBookId = "1";
  });

  it("ignores a stale fetch for the previous book that resolves after navigating to a new one", async () => {
    const calls: Array<(b: Book) => void> = [];
    (api.getBook as jest.Mock).mockImplementation(
      () => new Promise<Book>((resolve) => calls.push(resolve)),
    );

    const { rerender } = render(<BookDetailPage />);
    expect(api.getBook).toHaveBeenCalledTimes(1);

    // The user navigates to a different book before book #1's fetch (call
    // #0) resolves — the page component persists, only bookId changes.
    mockBookId = "2";
    rerender(<BookDetailPage />);
    expect(api.getBook).toHaveBeenCalledTimes(2);

    // The new book's fetch (call #1) resolves first.
    await act(async () => {
      calls[1](book({ id: 2, title: "Dune Messiah" }));
    });
    expect(screen.getAllByText("Dune Messiah").length).toBeGreaterThan(0);

    // Book #1's stale fetch (call #0) finally resolves — it must not
    // clobber the book #2 page now on screen.
    await act(async () => {
      calls[0](book({ id: 1, title: "Dune" }));
    });
    expect(screen.getAllByText("Dune Messiah").length).toBeGreaterThan(0);
    expect(screen.queryByText("Dune")).not.toBeInTheDocument();
  });
});
