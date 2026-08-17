import { render, screen, waitFor } from "@testing-library/react";
import { AdminGuard } from "./AdminGuard";

const push = jest.fn();
jest.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

describe("AdminGuard", () => {
  beforeEach(() => {
    localStorage.clear();
    push.mockClear();
  });

  it("redirects to /login when no token is stored", async () => {
    render(
      <AdminGuard>
        <div>secret</div>
      </AdminGuard>,
    );

    await waitFor(() => expect(push).toHaveBeenCalledWith("/login"));
    expect(screen.queryByText("secret")).not.toBeInTheDocument();
  });

  it("redirects to /catalog when the stored user is not an admin", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    localStorage.setItem("bookshelf_user", JSON.stringify({ role: "user" }));

    render(
      <AdminGuard>
        <div>secret</div>
      </AdminGuard>,
    );

    await waitFor(() => expect(push).toHaveBeenCalledWith("/catalog"));
  });

  it("redirects to /catalog when the stored user is malformed JSON", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    localStorage.setItem("bookshelf_user", "{not-json");

    render(
      <AdminGuard>
        <div>secret</div>
      </AdminGuard>,
    );

    await waitFor(() => expect(push).toHaveBeenCalledWith("/catalog"));
  });

  it("renders children once an admin token and user are present", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    localStorage.setItem("bookshelf_user", JSON.stringify({ role: "admin" }));

    render(
      <AdminGuard>
        <div>secret</div>
      </AdminGuard>,
    );

    await waitFor(() => expect(screen.getByText("secret")).toBeInTheDocument());
    expect(push).not.toHaveBeenCalled();
  });
});
