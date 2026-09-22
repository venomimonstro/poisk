import { NextRequest, NextResponse } from "next/server";

const internalBase = (process.env.API_INTERNAL_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

function allowed(path: string[], method: string) {
  if (path.length === 1 && path[0] === "sites") return method === "GET" || method === "POST";
  if (path.length === 1 && path[0] === "usage") return method === "GET";
  if (path.length < 3 || path[0] !== "sites" || !/^\d+$/.test(path[1])) return false;
  const action = path.slice(2).join("/");
  if (["verification", "verify", "sitemaps", "urls"].includes(action)) return method === "POST";
  if (["url-status", "metrics"].includes(action)) return method === "GET";
  return false;
}

async function proxy(request: NextRequest, path: string[]) {
  if (!allowed(path, request.method)) return NextResponse.json({ error: "not_found" }, { status: 404 });
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 8000);
  try {
    const headers = new Headers({ Accept: "application/json" });
    const cookie = request.headers.get("cookie");
    const csrf = request.headers.get("x-csrf-token");
    const contentType = request.headers.get("content-type");
    if (cookie) headers.set("Cookie", cookie);
    if (csrf) headers.set("X-CSRF-Token", csrf);
    if (contentType) headers.set("Content-Type", contentType);
    const body = request.method === "GET" ? undefined : await request.text();
    const query = request.nextUrl.search || "";
    const upstream = await fetch(`${internalBase}/api/portal/webmaster/${path.join("/")}${query}`, {
      method: request.method,
      headers,
      body,
      signal: controller.signal,
      cache: "no-store",
      redirect: "manual",
    });
    const text = await upstream.text();
    return new NextResponse(text || null, {
      status: upstream.status,
      headers: {
        "Content-Type": upstream.headers.get("Content-Type") || "application/json; charset=utf-8",
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return NextResponse.json({ error: "webmaster_backend_error" }, { status: 502 });
  } finally {
    clearTimeout(timer);
  }
}

export async function GET(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return proxy(request, (await context.params).path || []);
}
export async function POST(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return proxy(request, (await context.params).path || []);
}
