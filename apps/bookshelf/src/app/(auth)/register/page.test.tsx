import { render, screen } from "@testing-library/react";
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

async function fillDetails(
  user: ReturnType<typeof userEvent.setup>,
  password: string,
  confirmPassword: string,
) {
  await user.type(screen.getByLabelText("Name"), "Ada Lovelace");
  await user.type(screen.getByLabelText("Email"), "ada@example.com");
  await user.type(screen.getByLabelText("Password"), password);
  await user.type(screen.getByLabelText("Confirm password"), confirmPassword);
  await user.click(screen.getByRole("button", { name: "Continue" }));
}

describe("RegisterPage submit-time password errors", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (api.registrationRequirements as jest.Mock).mockResolvedValue({
      require_phone: false,
    });
  });

  it("does not render a prose error for a too-short password, and stays on the details step", async () => {
    const user = userEvent.setup();
    render(<RegisterPage />);

    await fillDetails(user, "Short1A", "Short1A");

    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.queryByText(/must be at least/i)).not.toBeInTheDocument();
    expect(api.sendRegisterEmailOTP).not.toHaveBeenCalled();
  });

  it("does not render a prose error for a common password, and stays on the details step", async () => {
    const user = userEvent.setup();
    render(<RegisterPage />);

    await fillDetails(user, "Bookshelf123", "Bookshelf123");

    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(screen.queryByText(/too common/i)).not.toBeInTheDocument();
    expect(api.sendRegisterEmailOTP).not.toHaveBeenCalled();
  });

  it("does not render a prose error for mismatched passwords, and stays on the details step", async () => {
    const user = userEvent.setup();
    render(<RegisterPage />);

    await fillDetails(user, "Str0ngPassw0rd!", "Str0ngPassw0rdX!");

    expect(screen.getByLabelText("Name")).toBeInTheDocument();
    expect(
      screen.queryByText(/passwords do not match/i),
    ).not.toBeInTheDocument();
    expect(api.sendRegisterEmailOTP).not.toHaveBeenCalled();
  });

  it("still renders a prose error for a genuine backend failure", async () => {
    const user = userEvent.setup();
    (api.sendRegisterEmailOTP as jest.Mock).mockRejectedValue(
      new Error("Email already registered"),
    );
    render(<RegisterPage />);

    await fillDetails(user, "Str0ngPassw0rd!", "Str0ngPassw0rd!");

    expect(
      await screen.findByText("Email already registered"),
    ).toBeInTheDocument();
  });
});
