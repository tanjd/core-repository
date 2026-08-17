import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WaitlistButton } from "./WaitlistButton";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => ({
  api: {
    getWaitlistStatus: jest.fn(),
    joinWaitlist: jest.fn(),
    leaveWaitlist: jest.fn(),
  },
}));

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

describe("WaitlistButton", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("renders nothing while the initial status is loading", () => {
    (api.getWaitlistStatus as jest.Mock).mockReturnValue(
      new Promise(() => undefined),
    );

    const { container } = render(<WaitlistButton copyId={1} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("shows 'Join Waitlist' when the user is not on the waitlist", async () => {
    (api.getWaitlistStatus as jest.Mock).mockResolvedValue({
      count: 0,
      on_waitlist: false,
    });

    render(<WaitlistButton copyId={1} />);

    expect(
      await screen.findByRole("button", { name: /Join Waitlist/ }),
    ).toBeInTheDocument();
  });

  it("shows the waiting count and 'Leave Waitlist' when already on it", async () => {
    (api.getWaitlistStatus as jest.Mock).mockResolvedValue({
      count: 2,
      on_waitlist: true,
    });

    render(<WaitlistButton copyId={1} />);

    expect(await screen.findByText("2 waiting")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Leave Waitlist/ }),
    ).toBeInTheDocument();
  });

  it("joins the waitlist when clicked", async () => {
    const user = userEvent.setup();
    (api.getWaitlistStatus as jest.Mock).mockResolvedValue({
      count: 0,
      on_waitlist: false,
    });
    (api.joinWaitlist as jest.Mock).mockResolvedValue(undefined);

    render(<WaitlistButton copyId={1} />);
    await user.click(
      await screen.findByRole("button", { name: /Join Waitlist/ }),
    );

    await waitFor(() => expect(api.joinWaitlist).toHaveBeenCalledWith(1));
    expect(
      await screen.findByRole("button", { name: /Leave Waitlist/ }),
    ).toBeInTheDocument();
  });

  it("leaves the waitlist when clicked", async () => {
    const user = userEvent.setup();
    (api.getWaitlistStatus as jest.Mock).mockResolvedValue({
      count: 1,
      on_waitlist: true,
    });
    (api.leaveWaitlist as jest.Mock).mockResolvedValue(undefined);

    render(<WaitlistButton copyId={1} />);
    await user.click(
      await screen.findByRole("button", { name: /Leave Waitlist/ }),
    );

    await waitFor(() => expect(api.leaveWaitlist).toHaveBeenCalledWith(1));
    expect(
      await screen.findByRole("button", { name: /Join Waitlist/ }),
    ).toBeInTheDocument();
  });
});
