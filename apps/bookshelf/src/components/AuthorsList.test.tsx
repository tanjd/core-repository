import { render, screen } from "@testing-library/react";
import { AuthorsList } from "./AuthorsList";
import type { AuthorSummary } from "@/lib/types";

describe("AuthorsList", () => {
  it("skips a row for an empty author string", () => {
    const authors: AuthorSummary[] = [
      { author: "", book_count: 1, covers: [] },
      { author: "Frank Herbert", book_count: 2, covers: [] },
    ];

    render(<AuthorsList authors={authors} />);

    expect(
      screen.getByRole("link", { name: /Frank Herbert/ }),
    ).toBeInTheDocument();
    // No sticky-letter header or link should be rendered for the blank author.
    expect(screen.queryByText("#")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });

  it("bakes fromHref into each author link as ?from= so 'Catalog' can restore this view", () => {
    const authors: AuthorSummary[] = [
      { author: "Frank Herbert", book_count: 2, covers: [] },
    ];

    render(
      <AuthorsList
        authors={authors}
        fromHref="/catalog?view=authors&authorQ=herb"
      />,
    );

    expect(screen.getByRole("link", { name: /Frank Herbert/ })).toHaveAttribute(
      "href",
      `/authors/Frank%20Herbert?from=${encodeURIComponent("/catalog?view=authors&authorQ=herb")}`,
    );
  });

  it("omits ?from= when no fromHref is given", () => {
    const authors: AuthorSummary[] = [
      { author: "Frank Herbert", book_count: 2, covers: [] },
    ];

    render(<AuthorsList authors={authors} />);

    expect(screen.getByRole("link", { name: /Frank Herbert/ })).toHaveAttribute(
      "href",
      "/authors/Frank%20Herbert",
    );
  });

  it("shows a fallback cover for an author whose books have no cover_url", () => {
    const authors: AuthorSummary[] = [
      { author: "Frank Herbert", book_count: 2, covers: [] },
    ];

    render(<AuthorsList authors={authors} />);

    // BookCoverFallback renders as an <img>-role SVG with a descriptive label.
    expect(
      screen.getByRole("img", { name: /Cover placeholder for Frank Herbert/ }),
    ).toBeInTheDocument();
  });
});
