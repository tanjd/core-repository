import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ToReadButton } from "./ToReadButton";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    addToRead: jest.fn(),
    removeToRead: jest.fn(),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

describe("ToReadButton", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders outline state with an accessible name disclosing the book", () => {
    render(<ToReadButton bookId={1} bookTitle="Deep Work" saved={false} />);

    const button = screen.getByRole("button", {
      name: "Save Deep Work to read later",
    });
    expect(button).toHaveAttribute("aria-pressed", "false");
  });

  it("renders the filled/saved state with a name disclosing that too", () => {
    render(<ToReadButton bookId={1} bookTitle="Deep Work" saved={true} />);

    const button = screen.getByRole("button", {
      name: "Remove Deep Work from your to-read list",
    });
    expect(button).toHaveAttribute("aria-pressed", "true");
  });

  it("optimistically flips state before the request resolves", async () => {
    const user = userEvent.setup();
    let resolveAdd: () => void = () => undefined;
    (api.addToRead as jest.Mock).mockReturnValue(
      new Promise<void>((resolve) => {
        resolveAdd = resolve;
      }),
    );

    render(<ToReadButton bookId={1} bookTitle="Deep Work" saved={false} />);

    await user.click(
      screen.getByRole("button", { name: "Save Deep Work to read later" }),
    );

    expect(
      screen.getByRole("button", {
        name: "Remove Deep Work from your to-read list",
      }),
    ).toHaveAttribute("aria-pressed", "true");

    resolveAdd();
    await waitFor(() => expect(api.addToRead).toHaveBeenCalledWith(1));
  });

  it("reverts state and surfaces a non-blocking error on failure", async () => {
    const user = userEvent.setup();
    (api.addToRead as jest.Mock).mockRejectedValue(new Error("network down"));
    const { toast } = jest.requireMock("sonner") as {
      toast: { error: jest.Mock };
    };

    render(<ToReadButton bookId={1} bookTitle="Deep Work" saved={false} />);

    await user.click(
      screen.getByRole("button", { name: "Save Deep Work to read later" }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Save Deep Work to read later" }),
      ).toHaveAttribute("aria-pressed", "false"),
    );
    expect(toast.error).toHaveBeenCalledWith("network down");
  });

  it("removes an existing saved entry when tapped", async () => {
    const user = userEvent.setup();
    (api.removeToRead as jest.Mock).mockResolvedValue(undefined);

    render(<ToReadButton bookId={1} bookTitle="Deep Work" saved={true} />);

    await user.click(
      screen.getByRole("button", {
        name: "Remove Deep Work from your to-read list",
      }),
    );

    await waitFor(() => expect(api.removeToRead).toHaveBeenCalledWith(1));
    expect(
      screen.getByRole("button", { name: "Save Deep Work to read later" }),
    ).toBeInTheDocument();
  });
});
