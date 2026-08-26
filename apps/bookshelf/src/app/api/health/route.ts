const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8000";

export async function GET() {
  try {
    const res = await fetch(`${BACKEND_URL}/health`, { cache: "no-store" });
    if (!res.ok) {
      return Response.json(
        { status: "error", message: "backend unreachable" },
        { status: 503 },
      );
    }

    const body: unknown = await res.json();
    return Response.json(body);
  } catch {
    return Response.json(
      { status: "error", message: "backend unreachable" },
      { status: 503 },
    );
  }
}
