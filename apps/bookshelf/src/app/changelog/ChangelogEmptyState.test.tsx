import { render, screen } from "@testing-library/react";

import { ChangelogEmptyState } from "@/app/changelog/ChangelogEmptyState";

describe("ChangelogEmptyState", () => {
  it("explains when release notes are unavailable", () => {
    render(<ChangelogEmptyState />);

    expect(screen.getByText("Release notes unavailable")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to catalog" }),
    ).toHaveAttribute("href", "/catalog");
  });
});
