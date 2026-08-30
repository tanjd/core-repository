import { act } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RegisterPage from "./page";
import { api } from "@/lib/api";

jest.mock("@/lib/api", () => {
  const actual = jest.requireActual("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      registrationRequirements: jest.fn(),
      sendRegisterEmailOTP: jest.fn(),
      verifyRegisterEmailOTP: jest.fn(),
      validateInviteCode: jest.fn(),
    },
  };
});

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

const push = jest.fn();
const replace = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace }),
}));

// A single big advanceTimersByTimeAsync(30_000) doesn't reliably chain
// through all 30 of the countdown's re-armed setTimeout calls, since each
// new timeout is only scheduled once React commits the re-render from the
// previous tick's state update. Ticking one second at a time, with an
// await between each, gives that commit+effect cycle room to run.
async function advanceSeconds(seconds: number) {
  for (let i = 0; i < seconds; i++) {
    await act(async () => {
      await jest.advanceTimersByTimeAsync(1000);
    });
  }
}

async function fillDetailsAndSubmit(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Name"), "Ada Lovelace");
  await user.type(screen.getByLabelText("Email"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), "Str0ngPassw0rd!");
  await user.type(screen.getByLabelText("Confirm password"), "Str0ngPassw0rd!");
  await user.click(screen.getByRole("button", { name: "Continue" }));
}

describe("RegisterPage resend cooldown", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers({ legacyFakeTimers: false });
    (api.registrationRequirements as jest.Mock).mockResolvedValue({
      require_phone: false,
    });
    (api.sendRegisterEmailOTP as jest.Mock).mockResolvedValue({});
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("disables Resend code immediately on reaching the verify-email step, counting down from 30", async () => {
    const user = userEvent.setup({ delay: null });
    render(<RegisterPage />);

    await fillDetailsAndSubmit(user);

    const resendButton = await screen.findByRole("button", {
      name: "Resend in 0:30",
    });
    expect(resendButton).toBeDisabled();
  });

  it("re-enables as 'Resend code' after the 30-second cooldown elapses", async () => {
    const user = userEvent.setup({ delay: null });
    render(<RegisterPage />);

    await fillDetailsAndSubmit(user);
    await screen.findByRole("button", { name: "Resend in 0:30" });

    await advanceSeconds(30);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Resend code" })).toBeEnabled(),
    );
  });

  it("restarts the countdown at 30 after a successful resend", async () => {
    const user = userEvent.setup({ delay: null });
    render(<RegisterPage />);

    await fillDetailsAndSubmit(user);
    await screen.findByRole("button", { name: "Resend in 0:30" });

    await advanceSeconds(30);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Resend code" })).toBeEnabled(),
    );

    await user.click(screen.getByRole("button", { name: "Resend code" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Resend in 0:30" }),
      ).toBeDisabled(),
    );
  });
});
