import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecommendButton } from "./RecommendButton";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    recommendBook: jest.fn(),
    unrecommendBook: jest.fn(),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

describe("RecommendButton", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders outline state with an accessible name disclosing the book and 'recommend'", () => {
    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={false}
        count={2}
      />,
    );

    const button = screen.getByRole("button", { name: "Recommend Deep Work" });
    expect(button).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders the filled/recommended state with a name disclosing that too", () => {
    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={true}
        count={3}
      />,
    );

    const button = screen.getByRole("button", {
      name: "Remove your recommendation for Deep Work",
    });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("hides the count when it is zero", () => {
    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={false}
        count={0}
      />,
    );
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("optimistically flips state and count before the request resolves", async () => {
    const user = userEvent.setup();
    let resolveRecommend: () => void = () => undefined;
    (api.recommendBook as jest.Mock).mockReturnValue(
      new Promise<void>((resolve) => {
        resolveRecommend = resolve;
      }),
    );

    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={false}
        count={0}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Recommend Deep Work" }),
    );

    // Flipped immediately, before the mocked request resolves.
    expect(
      screen.getByRole("button", {
        name: "Remove your recommendation for Deep Work",
      }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("1")).toBeInTheDocument();

    resolveRecommend();
    await waitFor(() => expect(api.recommendBook).toHaveBeenCalledWith(1));
  });

  it("reverts state and count and surfaces a non-blocking error on failure", async () => {
    const user = userEvent.setup();
    (api.recommendBook as jest.Mock).mockRejectedValue(
      new Error("network down"),
    );
    const { toast } = jest.requireMock("sonner") as {
      toast: { error: jest.Mock };
    };

    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={false}
        count={0}
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Recommend Deep Work" }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Recommend Deep Work" }),
      ).toHaveAttribute("aria-pressed", "false"),
    );
    expect(screen.queryByText("1")).not.toBeInTheDocument();
    expect(toast.error).toHaveBeenCalledWith("network down");
  });

  it("removes an existing recommendation when tapped", async () => {
    const user = userEvent.setup();
    (api.unrecommendBook as jest.Mock).mockResolvedValue(undefined);

    render(
      <RecommendButton
        bookId={1}
        bookTitle="Deep Work"
        recommended={true}
        count={1}
      />,
    );

    await user.click(
      screen.getByRole("button", {
        name: "Remove your recommendation for Deep Work",
      }),
    );

    await waitFor(() => expect(api.unrecommendBook).toHaveBeenCalledWith(1));
    expect(
      screen.getByRole("button", { name: "Recommend Deep Work" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("1")).not.toBeInTheDocument();
  });
});
