import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProfileForm } from "./ProfileForm";
import { api } from "@/lib/api";
import { openNewTab } from "@/lib/navigate";
import type { User, VerificationStatus } from "@/lib/types";

// Fake tab returned by openNewTab: a plain object with a settable
// location.href (asserted on below) and a spyable close(), standing in for
// the real popup window ProfileForm redirects once the link token arrives.
const fakeTab = { location: { href: "" }, close: jest.fn() };
jest.mock("@/lib/navigate", () => ({
  openNewTab: jest.fn(),
}));

// jsdom has no ResizeObserver — Radix's Tabs/Switch primitives need one.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
global.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver;

const push = jest.fn();
// A stable object, not a fresh literal per call — real Next.js memoizes the
// router, and ProfileForm's fetch effect depends on [router], so a
// non-stable mock would re-fire it (and reset in-progress form state) on
// every render.
const mockRouter = { push };
jest.mock("next/navigation", () => ({
  useRouter: () => mockRouter,
}));

jest.mock("@/lib/api", () => {
  const actual = jest.requireActual("@/lib/api");
  return {
    ...actual,
    api: {
      me: jest.fn(),
      myVerificationStatus: jest.fn(),
      updateMe: jest.fn(),
      linkTelegramToken: jest.fn(),
      unlinkTelegram: jest.fn(),
      sendTelegramTestNotification: jest.fn(),
    },
  };
});

jest.mock("sonner", () => ({
  toast: { success: jest.fn(), error: jest.fn() },
}));

// InviteLinkCard lives on a lazily-mounted tab (Radix Tabs only renders the
// active TabsContent), so it's never reached by these tests — stubbed only
// so a stray render doesn't fail on its own unmocked api calls.
jest.mock("@/components/InviteLinkCard", () => ({
  InviteLinkCard: () => null,
}));

const verificationStatus: VerificationStatus = { eligible: true, factors: [] };

function baseUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    name: "Ada",
    email: "ada@example.com",
    phone: "",
    verified: true,
    phone_verified: false,
    suspended: false,
    pending_approval: false,
    role: "user",
    created_at: "2026-01-01T00:00:00Z",
    google_books_key_configured: false,
    email_notifications_enabled: true,
    monthly_digest_enabled: true,
    telegram_linked: false,
    telegram_notifications_enabled: true,
    ...overrides,
  };
}

describe("ProfileForm — Telegram", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    fakeTab.location.href = "";
    (openNewTab as jest.Mock).mockReturnValue(fakeTab);
    localStorage.setItem("bookshelf_token", "test-token");
    (api.myVerificationStatus as jest.Mock).mockResolvedValue(
      verificationStatus,
    );
  });

  // Connect/Disconnect live on the Integrations tab; the notifications
  // toggle lives on the Profile tab (disabled, rather than hidden, when not
  // linked) — see ProfileForm.tsx's tab regroup.
  async function goToIntegrationsTab(user: ReturnType<typeof userEvent.setup>) {
    await user.click(await screen.findByRole("tab", { name: "Integrations" }));
  }

  it("shows a Connect button, and a disabled toggle, when Telegram isn't linked", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(baseUser());

    render(<ProfileForm />);
    await goToIntegrationsTab(user);

    expect(
      await screen.findByRole("button", { name: "Connect" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Disconnect Telegram" }),
    ).not.toBeInTheDocument();
  });

  it("disables the notifications toggle when Telegram isn't linked", async () => {
    (api.me as jest.Mock).mockResolvedValue(baseUser());

    render(<ProfileForm />);

    expect(
      await screen.findByRole("switch", {
        name: /telegram notifications/i,
      }),
    ).toBeDisabled();
  });

  it("shows the notifications toggle enabled and Disconnect, not Connect, once linked", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(
      baseUser({ telegram_linked: true, telegram_notifications_enabled: true }),
    );

    render(<ProfileForm />);

    expect(
      await screen.findByRole("switch", {
        name: "Disable Telegram notifications",
      }),
    ).toBeEnabled();

    await goToIntegrationsTab(user);
    expect(
      screen.getByRole("button", { name: "Disconnect Telegram" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Connect" }),
    ).not.toBeInTheDocument();
  });

  it("redirects to the bot deep link with the minted token and bot username", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(baseUser());
    (api.linkTelegramToken as jest.Mock).mockResolvedValue({
      token: "abc123",
      bot_username: "bookshelfbot",
    });

    render(<ProfileForm />);
    await goToIntegrationsTab(user);
    await user.click(await screen.findByRole("button", { name: "Connect" }));

    await waitFor(() =>
      expect(fakeTab.location.href).toBe(
        "https://t.me/bookshelfbot?start=abc123",
      ),
    );
  });

  it("shows an error and does not redirect when no bot is configured", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(baseUser());
    (api.linkTelegramToken as jest.Mock).mockResolvedValue({
      token: "abc123",
      bot_username: "",
    });
    const { toast } = jest.requireMock("sonner") as {
      toast: { error: jest.Mock };
    };

    render(<ProfileForm />);
    await goToIntegrationsTab(user);
    await user.click(await screen.findByRole("button", { name: "Connect" }));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "Telegram isn't configured on this deployment",
      ),
    );
    expect(fakeTab.close).toHaveBeenCalled();
  });

  it("disconnects, disabling the toggle and showing Connect again", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(
      baseUser({ telegram_linked: true, telegram_notifications_enabled: true }),
    );
    (api.unlinkTelegram as jest.Mock).mockResolvedValue(undefined);

    render(<ProfileForm />);
    await goToIntegrationsTab(user);
    await user.click(
      await screen.findByRole("button", { name: "Disconnect Telegram" }),
    );

    await waitFor(() => expect(api.unlinkTelegram).toHaveBeenCalled());
    expect(
      await screen.findByRole("button", { name: "Connect" }),
    ).toBeInTheDocument();

    await user.click(await screen.findByRole("tab", { name: "Profile" }));
    expect(
      await screen.findByRole("switch", { name: /telegram notifications/i }),
    ).toBeDisabled();
  });

  it("sends a test notification and shows a success toast", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(
      baseUser({ telegram_linked: true, telegram_notifications_enabled: true }),
    );
    (api.sendTelegramTestNotification as jest.Mock).mockResolvedValue(
      undefined,
    );
    const { toast } = jest.requireMock("sonner") as {
      toast: { success: jest.Mock };
    };

    render(<ProfileForm />);
    await user.click(
      await screen.findByRole("button", { name: "Send test message" }),
    );

    await waitFor(() =>
      expect(api.sendTelegramTestNotification).toHaveBeenCalled(),
    );
    expect(toast.success).toHaveBeenCalledWith(
      "Test message sent — check Telegram",
    );
  });

  it("shows an error toast when the test notification fails to deliver", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(
      baseUser({ telegram_linked: true, telegram_notifications_enabled: true }),
    );
    (api.sendTelegramTestNotification as jest.Mock).mockRejectedValue(
      new Error(
        "could not reach Telegram — check the bot is still linked and try again",
      ),
    );
    const { toast } = jest.requireMock("sonner") as {
      toast: { error: jest.Mock };
    };

    render(<ProfileForm />);
    await user.click(
      await screen.findByRole("button", { name: "Send test message" }),
    );

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        "could not reach Telegram — check the bot is still linked and try again",
      ),
    );
  });

  it("omits telegram_notifications_enabled from the save payload when not linked", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(baseUser());
    (api.updateMe as jest.Mock).mockResolvedValue(baseUser());

    render(<ProfileForm />);
    await user.click(
      await screen.findByRole("button", { name: "Save changes" }),
    );

    await waitFor(() => expect(api.updateMe).toHaveBeenCalled());
    const payload = (api.updateMe as jest.Mock).mock.calls[0][0];
    expect(payload.telegram_notifications_enabled).toBeUndefined();
  });

  it("includes the toggle's value in the save payload once linked", async () => {
    const user = userEvent.setup();
    (api.me as jest.Mock).mockResolvedValue(
      baseUser({ telegram_linked: true, telegram_notifications_enabled: true }),
    );
    (api.updateMe as jest.Mock).mockResolvedValue(baseUser());

    render(<ProfileForm />);
    const sw = await screen.findByRole("switch", {
      name: "Disable Telegram notifications",
    });
    await user.click(sw);
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() => expect(api.updateMe).toHaveBeenCalled());
    const payload = (api.updateMe as jest.Mock).mock.calls[0][0];
    expect(payload.telegram_notifications_enabled).toBe(false);
  });
});
