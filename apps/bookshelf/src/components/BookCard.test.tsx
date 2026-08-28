import { render, screen } from "@testing-library/react";
import { BookCard } from "./BookCard";
import type { Book } from "@/lib/types";

// Substituting a plain <img> for next/image is the whole point of the
// mock — jsdom has no image optimizer to run, and asserting on
// container.querySelector("img") is how these tests distinguish the
// real cover from the SVG fallback. next/image's lint rule doesn't
// apply here.
jest.mock("next/image", () => ({
  __esModule: true,
  default: (props: Record<string, unknown>) => {
    const imgProps = { ...props };
    delete imgProps.fill;

    // eslint-disable-next-line @next/next/no-img-element
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

    // Without a cover image, the title and author also appear in the
    // generated SVG fallback's text, so at least one match — not exactly
    // one — is what matters.
    expect(
      screen.getAllByText("The Go Programming Language").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("Donovan & Kernighan").length).toBeGreaterThan(
      0,
    );
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

  it("links to the detail page directly when no catalogHref is provided", () => {
    const { container } = render(<BookCard book={baseBook} />);

    expect(container.querySelector("a")).toHaveAttribute("href", "/catalog/1");
  });

  it("includes catalogHref as a from param in the detail link", () => {
    const { container } = render(
      <BookCard book={baseBook} catalogHref="/catalog?page=3&sort=newest" />,
    );

    expect(container.querySelector("a")).toHaveAttribute(
      "href",
      `/catalog/1?from=${encodeURIComponent("/catalog?page=3&sort=newest")}`,
    );
  });

  it("renders a generated cover fallback when there is no cover image", () => {
    const { container } = render(<BookCard book={baseBook} />);

    expect(container.querySelector("img")).not.toBeInTheDocument();
    expect(container.querySelector("svg")).toBeInTheDocument();
  });

  it("renders the cover image when a cover_url is present", () => {
    const { container } = render(
      <BookCard
        book={{ ...baseBook, cover_url: "https://example.com/cover.jpg" }}
      />,
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "https://example.com/cover.jpg",
    );
  });

  it("renders a recommend toggle reflecting the viewer's state and count, but no facepile", () => {
    render(
      <BookCard
        book={{
          ...baseBook,
          recommendation_count: 4,
          your_recommendation: true,
        }}
      />,
    );

    expect(
      screen.getByRole("button", {
        name: "Remove your recommendation for The Go Programming Language",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    // "Facepile stays off the card" — count + toggle only.
    expect(screen.queryByText("Recommended by")).not.toBeInTheDocument();
  });
});
