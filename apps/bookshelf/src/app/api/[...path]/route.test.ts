/**
 * @jest-environment node
 *
 * next/server's NextRequest extends the global Request class, which jsdom
 * (this project's default test environment) doesn't provide — Route
 * Handlers need the node environment's native fetch/Request/Response.
 */
import { NextRequest } from "next/server";

describe("api proxy route", () => {
  const originalFetch = global.fetch;
  const originalBackendUrl = process.env.BACKEND_URL;

  afterEach(() => {
    global.fetch = originalFetch;
    if (originalBackendUrl === undefined) {
      delete process.env.BACKEND_URL;
    } else {
      process.env.BACKEND_URL = originalBackendUrl;
    }
    jest.resetModules();
  });

  // BACKEND_URL is read once at module load, so each test that cares about
  // it sets the env var and re-imports the module fresh.
  async function loadRoute() {
    jest.resetModules();
    return import("./route");
  }

  it("forwards GET requests to BACKEND_URL with the joined path and query string", async () => {
    process.env.BACKEND_URL = "http://backend.internal:9000";
    const { GET } = await loadRoute();
    global.fetch = jest.fn().mockResolvedValue(
      new Response("ok", {
        status: 200,
        headers: { "content-type": "text/plain" },
      }),
    );

    const req = new NextRequest("http://localhost:3000/api/books?q=go", {
      method: "GET",
    });
    const res = await GET(req, {
      params: Promise.resolve({ path: ["books"] }),
    });

    expect(global.fetch).toHaveBeenCalledWith(
      "http://backend.internal:9000/books?q=go",
      expect.objectContaining({ method: "GET" }),
    );
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toBe("text/plain");
  });

  it("falls back to http://localhost:8000 when BACKEND_URL is unset", async () => {
    delete process.env.BACKEND_URL;
    const { GET } = await loadRoute();
    global.fetch = jest
      .fn()
      .mockResolvedValue(new Response("ok", { status: 200 }));

    const req = new NextRequest("http://localhost:3000/api/books", {
      method: "GET",
    });
    await GET(req, { params: Promise.resolve({ path: ["books"] }) });

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8000/books",
      expect.anything(),
    );
  });

  it("joins multi-segment paths", async () => {
    process.env.BACKEND_URL = "http://backend.internal:9000";
    const { GET } = await loadRoute();
    global.fetch = jest
      .fn()
      .mockResolvedValue(new Response("ok", { status: 200 }));

    const req = new NextRequest("http://localhost:3000/api/loan-requests/5", {
      method: "GET",
    });
    await GET(req, {
      params: Promise.resolve({ path: ["loan-requests", "5"] }),
    });

    expect(global.fetch).toHaveBeenCalledWith(
      "http://backend.internal:9000/loan-requests/5",
      expect.anything(),
    );
  });

  it("forwards the method and strips the host header", async () => {
    process.env.BACKEND_URL = "http://backend.internal:9000";
    const { POST } = await loadRoute();
    global.fetch = jest
      .fn()
      .mockResolvedValue(new Response(null, { status: 201 }));

    const req = new NextRequest("http://localhost:3000/api/books", {
      method: "POST",
      headers: { host: "localhost:3000", authorization: "Bearer abc123" },
      body: JSON.stringify({ title: "New Book" }),
    });
    const res = await POST(req, {
      params: Promise.resolve({ path: ["books"] }),
    });

    const [, init] = (global.fetch as jest.Mock).mock.calls[0];
    expect(init.method).toBe("POST");
    expect(init.headers.get("host")).toBeNull();
    expect(init.headers.get("authorization")).toBe("Bearer abc123");
    expect(res.status).toBe(201);
  });

  it("propagates the upstream status code on error responses", async () => {
    process.env.BACKEND_URL = "http://backend.internal:9000";
    const { DELETE } = await loadRoute();
    global.fetch = jest
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: "not found" }), { status: 404 }),
      );

    const req = new NextRequest("http://localhost:3000/api/books/999", {
      method: "DELETE",
    });
    const res = await DELETE(req, {
      params: Promise.resolve({ path: ["books", "999"] }),
    });

    expect(res.status).toBe(404);
  });
});
