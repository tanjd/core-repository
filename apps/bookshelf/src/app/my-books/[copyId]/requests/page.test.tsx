import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CopyRequestsPage from "./page";
import { api } from "@/lib/api";
import type { LoanRequest } from "@/lib/types";

jest.mock("@/lib/api", () => ({
  api: {
    getLoanRequestsByCopy: jest.fn(),
    updateLoanRequest: jest.fn(),
    updateExpectedReturnDate: jest.fn(),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const push = jest.fn();
jest.mock("next/navigation", () => ({
  useParams: () => ({ copyId: "1" }),
  useRouter: () => ({ push }),
}));

function toDateOnly(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function pendingRequest(overrides: Partial<LoanRequest> = {}): LoanRequest {
  return {
    id: 1,
    copy_id: 1,
    borrower_id: 2,
    message: "",
    status: "pending",
    requested_at: "2026-01-01T00:00:00Z",
    expected_return_date: "2026-01-01T00:00:00Z",
    copy: { id: 1, book: { title: "Dune", author: "Frank Herbert" } },
    borrower: { id: 2, name: "Ada" },
    ...overrides,
  } as LoanRequest;
}

describe("Accept dialog return-date clamping", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.setItem("bookshelf_token", "test-token");
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("bumps a past proposed return date up to today instead of pre-filling a stale past date", async () => {
    // The request has sat pending long enough that its originally-proposed
    // return date is now in the past.
    const request = pendingRequest({
      expected_return_date: "2020-01-01T00:00:00Z",
    });
    (api.getLoanRequestsByCopy as jest.Mock).mockResolvedValue([request]);

    const user = userEvent.setup();
    render(<CopyRequestsPage />);

    const acceptButtons = await screen.findAllByRole("button", {
      name: "Accept",
    });
    await user.click(acceptButtons[0]);

    const dateInput = (await screen.findByLabelText(
      "Return by",
    )) as HTMLInputElement;
    const today = toDateOnly(new Date());
    expect(dateInput.value).toBe(today);
    // The input's own min shouldn't be violated by its own seeded value.
    expect(dateInput.value >= dateInput.min).toBe(true);
  });

  it("leaves a future proposed return date untouched", async () => {
    const future = new Date();
    future.setDate(future.getDate() + 14);
    const futureDateOnly = toDateOnly(future);
    const request = pendingRequest({
      expected_return_date: `${futureDateOnly}T00:00:00Z`,
    });
    (api.getLoanRequestsByCopy as jest.Mock).mockResolvedValue([request]);

    const user = userEvent.setup();
    render(<CopyRequestsPage />);

    const acceptButtons = await screen.findAllByRole("button", {
      name: "Accept",
    });
    await user.click(acceptButtons[0]);

    const dateInput = (await screen.findByLabelText(
      "Return by",
    )) as HTMLInputElement;
    expect(dateInput.value).toBe(futureDateOnly);
  });

  it("leaves the date blank when the borrower proposed none", async () => {
    const request = pendingRequest({ expected_return_date: "" });
    (api.getLoanRequestsByCopy as jest.Mock).mockResolvedValue([request]);

    const user = userEvent.setup();
    render(<CopyRequestsPage />);

    const acceptButtons = await screen.findAllByRole("button", {
      name: "Accept",
    });
    await user.click(acceptButtons[0]);

    const dateInput = (await screen.findByLabelText(
      "Return by",
    )) as HTMLInputElement;
    expect(dateInput.value).toBe("");
  });

  it("PATCHes the corrected date on confirm when a past date was clamped", async () => {
    const request = pendingRequest({
      expected_return_date: "2020-01-01T00:00:00Z",
    });
    (api.getLoanRequestsByCopy as jest.Mock).mockResolvedValue([request]);
    (api.updateLoanRequest as jest.Mock).mockResolvedValue({
      ...request,
      status: "accepted",
    });
    (api.updateExpectedReturnDate as jest.Mock).mockResolvedValue({
      ...request,
      status: "accepted",
    });

    const user = userEvent.setup();
    render(<CopyRequestsPage />);

    const acceptButtons = await screen.findAllByRole("button", {
      name: "Accept",
    });
    await user.click(acceptButtons[0]);

    await user.click(
      await screen.findByRole("button", { name: "Accept Request" }),
    );

    expect(api.updateLoanRequest).toHaveBeenCalledWith(request.id, {
      status: "accepted",
    });
    expect(api.updateExpectedReturnDate).toHaveBeenCalledWith(
      request.id,
      toDateOnly(new Date()),
    );
  });
});
