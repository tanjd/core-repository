import { render, screen } from "@testing-library/react";
import { AuthorsList } from "./AuthorsList";
import type { AuthorSummary } from "@/lib/types";

describe("AuthorsList", () => {
  it("skips a row for an empty author string", () => {
    const authors: AuthorSummary[] = [
      { author: "", book_count: 1 },
      { author: "Frank Herbert", book_count: 2 },
    ];

    render(<AuthorsList authors={authors} />);

    expect(screen.getByText("Frank Herbert")).toBeInTheDocument();
    // No sticky-letter header or link should be rendered for the blank author.
    expect(screen.queryByText("#")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link")).toHaveLength(1);
  });
});
