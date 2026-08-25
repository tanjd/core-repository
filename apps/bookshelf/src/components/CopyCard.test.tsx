import { render, screen } from "@testing-library/react";
import { CopyCard } from "./CopyCard";
import type { Copy } from "@/lib/types";

const baseCopy: Copy = {
  id: 1,
  book_id: 1,
  owner_id: 1,
  condition: "good",
  notes: "",
  status: "available",
};

describe("CopyCard", () => {
  it("renders condition and status", () => {
    render(<CopyCard copy={baseCopy} />);

    expect(screen.getByText("Good condition")).toBeInTheDocument();
    expect(screen.getByText("Available")).toBeInTheDocument();
  });

  it("shows the owner's name when present and not hidden", () => {
    render(<CopyCard copy={{ ...baseCopy, owner: { id: 2, name: "Ada" } }} />);

    expect(screen.getByText("Ada")).toBeInTheDocument();
  });

  it("shows 'Anonymous member' when hide_owner is set, even if a name is present", () => {
    render(
      <CopyCard
        copy={{ ...baseCopy, hide_owner: true, owner: { id: 2, name: "Ada" } }}
      />,
    );

    expect(screen.getByText("Anonymous member")).toBeInTheDocument();
    expect(screen.queryByText("Ada")).not.toBeInTheDocument();
  });

  it("prompts sign-in when no owner info is available", () => {
    render(<CopyCard copy={baseCopy} />);

    expect(
      screen.getByText("Sign in and verify to see who's sharing"),
    ).toBeInTheDocument();
  });

  it("renders notes when present", () => {
    render(<CopyCard copy={{ ...baseCopy, notes: "Slight water damage" }} />);

    expect(screen.getByText("Slight water damage")).toBeInTheDocument();
  });

  it("renders provided actions", () => {
    render(<CopyCard copy={baseCopy} actions={<button>Request</button>} />);

    expect(screen.getByRole("button", { name: "Request" })).toBeInTheDocument();
  });

  it.each([
    ["requested", "Requested"],
    ["loaned", "On Loan"],
    ["unavailable", "Unavailable"],
  ] as const)("labels status %s as %s", (status, label) => {
    render(<CopyCard copy={{ ...baseCopy, status }} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("marks auto-approve available copies with an inline hint", () => {
    render(<CopyCard copy={{ ...baseCopy, auto_approve: true }} />);
    expect(screen.getByText(/Instant approval/)).toBeInTheDocument();
  });

  it("shows a 'Best pick' hint when highlighted and available", () => {
    render(<CopyCard copy={baseCopy} highlighted />);
    expect(screen.getByText(/Best pick/)).toBeInTheDocument();
  });
});
