import { validatePassword, api } from "./api";

describe("validatePassword", () => {
  it("accepts a password meeting all complexity rules", () => {
    expect(validatePassword("Passw0rd")).toBeNull();
  });

  it("rejects passwords under 8 characters", () => {
    expect(validatePassword("Aa1")).toBe(
      "Password must be at least 8 characters",
    );
  });

  it("rejects passwords without an uppercase letter", () => {
    expect(validatePassword("password1")).toBe(
      "Password must contain at least one uppercase letter",
    );
  });

  it("rejects passwords without a lowercase letter", () => {
    expect(validatePassword("PASSWORD1")).toBe(
      "Password must contain at least one lowercase letter",
    );
  });

  it("rejects passwords without a digit", () => {
    expect(validatePassword("Password")).toBe(
      "Password must contain at least one number",
    );
  });
});

describe("api request wrapper", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    localStorage.clear();
    global.fetch = jest.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("omits the Authorization header when no token is stored", async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ needs_setup: false }),
    });

    await api.setupStatus();

    const [, init] = (global.fetch as jest.Mock).mock.calls[0];
    expect(init.headers.Authorization).toBeUndefined();
  });

  it("attaches a Bearer Authorization header when a token is stored", async () => {
    localStorage.setItem("bookshelf_token", "abc123");
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ id: 1 }),
    });

    await api.me();

    const [, init] = (global.fetch as jest.Mock).mock.calls[0];
    expect(init.headers.Authorization).toBe("Bearer abc123");
  });

  it("returns undefined for an empty response body", async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      text: async () => "",
    });

    const result = await api.markAllRead();

    expect(result).toBeUndefined();
  });

  it("parses the JSON body on success", async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      text: async () => JSON.stringify({ needs_setup: true }),
    });

    const result = await api.setupStatus();

    expect(result).toEqual({ needs_setup: true });
  });

  it("throws the server-provided error message on a non-OK response", async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      statusText: "Bad Request",
      json: async () => ({ error: "invalid credentials" }),
    });

    await expect(
      api.login({ email: "a@example.com", password: "wrong" }),
    ).rejects.toThrow("invalid credentials");
  });

  it("falls back to the status text when the error body isn't JSON", async () => {
    (global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
      json: async () => {
        throw new Error("not json");
      },
    });

    await expect(api.setupStatus()).rejects.toThrow("Internal Server Error");
  });
});
