import { render, screen, waitFor } from "@testing-library/react";
import { SetupGuard } from "./SetupGuard";
import { api } from "@/lib/api";

const replace = jest.fn();
let pathname = "/catalog";
jest.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => pathname,
}));

jest.mock("@/lib/api", () => ({
  api: {
    setupStatus: jest.fn(),
  },
}));

describe("SetupGuard", () => {
  beforeEach(() => {
    replace.mockClear();
    jest.clearAllMocks();
    pathname = "/catalog";
  });

  it("redirects to /setup when setup is needed", async () => {
    (api.setupStatus as jest.Mock).mockResolvedValue({ needs_setup: true });

    render(
      <SetupGuard>
        <div>app</div>
      </SetupGuard>,
    );

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/setup"));
    expect(screen.queryByText("app")).not.toBeInTheDocument();
  });

  it("renders children once setup is confirmed complete", async () => {
    (api.setupStatus as jest.Mock).mockResolvedValue({ needs_setup: false });

    render(
      <SetupGuard>
        <div>app</div>
      </SetupGuard>,
    );

    await waitFor(() => expect(screen.getByText("app")).toBeInTheDocument());
    expect(replace).not.toHaveBeenCalled();
  });

  it("renders children if the setup-status check fails, rather than blocking the app", async () => {
    (api.setupStatus as jest.Mock).mockRejectedValue(new Error("network"));

    render(
      <SetupGuard>
        <div>app</div>
      </SetupGuard>,
    );

    await waitFor(() => expect(screen.getByText("app")).toBeInTheDocument());
    expect(replace).not.toHaveBeenCalled();
  });

  it("skips the check entirely when already on /setup", () => {
    pathname = "/setup";

    render(
      <SetupGuard>
        <div>setup page</div>
      </SetupGuard>,
    );

    expect(screen.getByText("setup page")).toBeInTheDocument();
    expect(api.setupStatus).not.toHaveBeenCalled();
  });
});
