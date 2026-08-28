import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecommendedBy } from "./RecommendedBy";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    getRecommendations: jest.fn(),
  },
}));

describe("RecommendedBy", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders nothing when nobody has recommended the book", async () => {
    (api.getRecommendations as jest.Mock).mockResolvedValue([]);

    const { container } = render(<RecommendedBy bookId={1} />);

    await waitFor(() => expect(api.getRecommendations).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while the list is loading, then shows avatars", async () => {
    (api.getRecommendations as jest.Mock).mockResolvedValue([
      { recommender_name: "Ada Lovelace", created_at: "2026-01-01T00:00:00Z" },
    ]);

    const { container } = render(<RecommendedBy bookId={1} />);
    expect(container).toBeEmptyDOMElement();

    await waitFor(() =>
      expect(screen.getByText("Recommended by")).toBeInTheDocument(),
    );
  });

  it("shows the first 3 recommenders with no overflow control at exactly 3", async () => {
    (api.getRecommendations as jest.Mock).mockResolvedValue([
      { recommender_name: "One", created_at: "2026-01-03T00:00:00Z" },
      { recommender_name: "Two", created_at: "2026-01-02T00:00:00Z" },
      { recommender_name: "Three", created_at: "2026-01-01T00:00:00Z" },
    ]);

    render(<RecommendedBy bookId={1} />);

    await waitFor(() =>
      expect(screen.getByText("Recommended by")).toBeInTheDocument(),
    );
    expect(screen.queryByText(/and \d+ other/)).not.toBeInTheDocument();
  });

  it("collapses beyond 3 into a focusable 'and N others' control that opens the full list", async () => {
    const user = userEvent.setup();
    (api.getRecommendations as jest.Mock).mockResolvedValue([
      { recommender_name: "One", created_at: "2026-01-04T00:00:00Z" },
      { recommender_name: "Two", created_at: "2026-01-03T00:00:00Z" },
      { recommender_name: "Three", created_at: "2026-01-02T00:00:00Z" },
      { recommender_name: "Four", created_at: "2026-01-01T00:00:00Z" },
    ]);

    render(<RecommendedBy bookId={1} />);

    const overflowButton = await screen.findByRole("button", {
      name: "and 1 other",
    });
    // Not decorative text — a real, keyboard-reachable control.
    expect(overflowButton.tagName).toBe("BUTTON");
    expect(screen.queryByText("Four")).not.toBeInTheDocument();

    await user.click(overflowButton);

    expect(await screen.findByText("Four")).toBeInTheDocument();
    expect(screen.getByText("One")).toBeInTheDocument();
  });

  it("falls back to an empty (not error) state if the fetch fails", async () => {
    (api.getRecommendations as jest.Mock).mockRejectedValue(
      new Error("network"),
    );

    const { container } = render(<RecommendedBy bookId={1} />);

    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });
});
