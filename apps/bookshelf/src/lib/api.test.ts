import { validatePassword, scorePasswordStrength, api } from "./api";

describe("validatePassword", () => {
  it("accepts a password meeting all complexity rules", () => {
    expect(validatePassword("Passw0rd1234")).toBeNull();
  });

  it("rejects passwords under 12 characters", () => {
    expect(validatePassword("Aa1")).toBe(
      "Password must be at least 12 characters",
    );
  });

  it("rejects passwords over 72 characters", () => {
    expect(validatePassword("Aa1" + "b".repeat(70))).toBe(
      "Password must be at most 72 characters",
    );
  });

  it("rejects passwords without an uppercase letter", () => {
    expect(validatePassword("password12345")).toBe(
      "Password must contain at least one uppercase letter",
    );
  });

  it("rejects passwords without a lowercase letter", () => {
    expect(validatePassword("PASSWORD12345")).toBe(
      "Password must contain at least one lowercase letter",
    );
  });

  it("rejects passwords without a digit", () => {
    expect(validatePassword("PasswordLetters")).toBe(
      "Password must contain at least one number",
    );
  });

  it("rejects a password pulled from the common-password denylist", () => {
    // Uppercase/lowercase/digit present (so it clears the composition
    // checks) but still matches the denylist case-insensitively.
    expect(validatePassword("Bookshelf123")).toBe(
      "This password is too common — please choose a stronger one",
    );
  });

  it("rejects passwords containing a disallowed name or email", () => {
    expect(validatePassword("MyNameIsAda123", ["Ada", "ada"])).toBe(
      "Password must not contain your name or email",
    );
  });

  it("ignores disallowed entries shorter than 3 characters", () => {
    expect(validatePassword("Passw0rd1234", ["Jo"])).toBeNull();
  });
});

describe("scorePasswordStrength", () => {
  it("scores an empty password as very weak", () => {
    expect(scorePasswordStrength("").score).toBe(0);
  });

  it("scores a common password as very weak even if long", () => {
    expect(scorePasswordStrength("bookshelf123").score).toBe(0);
  });

  it("scores a short simple password low", () => {
    expect(scorePasswordStrength("aaaaaaaaaaaa").score).toBeLessThanOrEqual(1);
  });

  it("scores a long, varied, non-sequential password high", () => {
    expect(
      scorePasswordStrength("Tr0mb0ne$Kayak!9").score,
    ).toBeGreaterThanOrEqual(3);
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
      // huma's ErrorModel puts the message in `detail`, not `error` — see
      // e.g. auth.go's huma.Error400BadRequest calls.
      json: async () => ({ detail: "invalid credentials" }),
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
