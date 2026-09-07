import { type NextRequest, NextResponse } from "next/server";

// Read at module load time (server startup) — picks up the runtime BACKEND_URL env var.
const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8000";

async function proxy(
  req: NextRequest,
  segments: string[],
): Promise<NextResponse> {
  const { search } = new URL(req.url);
  const target = `${BACKEND_URL}/${segments.join("/")}${search}`;

  const headers = new Headers(req.headers);
  headers.delete("host"); // don't forward the frontend's host header

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  const upstream = await fetch(target, {
    method: req.method,
    headers,
    body: hasBody ? req.body : undefined,
    // needed by Node.js fetch for streaming request bodies (e.g. file uploads)
    ...(hasBody && { duplex: "half" }),
  } as RequestInit);

  const resHeaders = new Headers();
  const ct = upstream.headers.get("content-type");
  if (ct) resHeaders.set("content-type", ct);
  // Every /api/* response is dynamic and often per-user (auth-scoped) — make
  // sure no intermediary (Cloudflare, a corporate proxy, the browser's own
  // HTTP cache) ever caches it, regardless of upstream/edge cache config.
  resHeaders.set("cache-control", "no-store");

  return new NextResponse(upstream.body, {
    status: upstream.status,
    headers: resHeaders,
  });
}

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  return proxy(req, (await params).path);
}

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  return proxy(req, (await params).path);
}

export async function PATCH(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  return proxy(req, (await params).path);
}

export async function DELETE(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  return proxy(req, (await params).path);
}
