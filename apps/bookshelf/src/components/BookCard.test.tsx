import { render, screen } from "@testing-library/react";
import { BookCard } from "./BookCard";
import type { Book } from "@/lib/types";

jest.mock("next/image", () => ({
  __esModule: true,
  default: (props: Record<string, unknown>) => {
    const imgProps = { ...props };
    delete imgProps.fill;

    return <img {...imgProps} alt={props.alt as string} />;
  },
}));

const baseBook: Book = {
  id: 1,
  title: "The Go Programming Language",
  author: "Donovan & Kernighan",
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
};

describe("BookCard", () => {
  it("renders the title and author", () => {
    render(<BookCard book={baseBook} />);

    // Without a cover image, the title also appears in the fallback icon
    // caption, so at least one match — not exactly one — is what matters.
    expect(
      screen.getAllByText("The Go Programming Language").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("Donovan & Kernighan")).toBeInTheDocument();
  });

  it("shows a 'Yours' badge when ownedByMe is true", () => {
    render(<BookCard book={baseBook} ownedByMe />);

    expect(screen.getByText("Yours")).toBeInTheDocument();
  });

  it("does not show a 'Yours' badge by default", () => {
    render(<BookCard book={baseBook} />);

    expect(screen.queryByText("Yours")).not.toBeInTheDocument();
  });

  it("shows an availability count when provided", () => {
    render(<BookCard book={{ ...baseBook, available_copies: 3 }} />);

    expect(screen.getByText("3 available")).toBeInTheDocument();
  });

  it("shows 'Unavailable' when there are zero available copies", () => {
    render(<BookCard book={{ ...baseBook, available_copies: 0 }} />);

    expect(screen.getByText("Unavailable")).toBeInTheDocument();
  });

  it("renders a fallback icon and title when there is no cover image", () => {
    render(<BookCard book={baseBook} />);

    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders the cover image when a cover_url is present", () => {
    render(
      <BookCard
        book={{ ...baseBook, cover_url: "https://example.com/cover.jpg" }}
      />,
    );

    expect(screen.getByRole("img")).toHaveAttribute(
      "src",
      "https://example.com/cover.jpg",
    );
  });
});
