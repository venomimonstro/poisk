import { NextRequest, NextResponse } from "next/server";

const internalBase = (process.env.API_INTERNAL_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams.get("q") || "";
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 4500);
  try {
    const upstream = await fetch(`${internalBase}/api/answer?q=${encodeURIComponent(q)}`, {
      signal: controller.signal,
      headers: { Accept: "application/json" },
      cache: "no-store",
    });
    const body = await upstream.text();
    return new NextResponse(body, {
      status: upstream.status,
      headers: {
        "Content-Type": upstream.headers.get("Content-Type") || "application/json; charset=utf-8",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return NextResponse.json({ error: "answer_backend_error" }, { status: 502 });
  } finally {
    clearTimeout(timer);
  }
}
