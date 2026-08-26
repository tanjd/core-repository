import { render, screen, waitFor } from "@testing-library/react";
import { BookshelfRow } from "./BookshelfRow";
import { api } from "@/lib/api";
import type { Book } from "@/lib/types";

jest.mock("@/lib/api", () => ({
  api: {
    getRecentBooks: jest.fn(),
  },
}));

jest.mock("next/image", () => ({
  __esModule: true,
  default: (props: Record<string, unknown>) => {
    const imgProps = { ...props };
    delete imgProps.fill;

    // eslint-disable-next-line @next/next/no-img-element
    return <img {...imgProps} alt={props.alt as string} />;
  },
}));

const mockGetRecentBooks = api.getRecentBooks as jest.MockedFunction<
  typeof api.getRecentBooks
>;

const recentBook: Book = {
  id: 42,
  title: "Recently Added Book",
  author: "Recent Author",
  isbn: "",
  ol_key: "",
  cover_url: "",
  description: "",
  publisher: "",
  published_date: "",
  page_count: 0,
  language: "",
  google_books_id: "",
  created_at: "",
  available_copies: 2,
};

describe("BookshelfRow", () => {
  beforeEach(() => {
    mockGetRecentBooks.mockResolvedValue([recentBook]);
  });

  it("links to the detail page directly when no catalogHref is provided", async () => {
    const { container } = render(
      <BookshelfRow limit={12} ownedBookIds={new Set()} />,
    );

    await waitFor(() => {
      expect(screen.getByText("Recently Added Book")).toBeInTheDocument();
    });

    expect(container.querySelector("a")).toHaveAttribute("href", "/catalog/42");
  });

  it("includes catalogHref as a from param in the detail link", async () => {
    const { container } = render(
      <BookshelfRow
        limit={12}
        ownedBookIds={new Set()}
        catalogHref="/catalog?page=3&sort=newest"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Recently Added Book")).toBeInTheDocument();
    });

    expect(container.querySelector("a")).toHaveAttribute(
      "href",
      `/catalog/42?from=${encodeURIComponent("/catalog?page=3&sort=newest")}`,
    );
  });
});
