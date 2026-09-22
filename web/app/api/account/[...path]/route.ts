import { NextRequest, NextResponse } from "next/server";

const internalBase = (process.env.API_INTERNAL_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const staticPaths = new Set(["register", "login", "me", "csrf", "sessions", "logout"]);

function allowed(path: string[], method: string) {
  if (path.length === 1 && staticPaths.has(path[0])) {
    if (["register", "login", "logout"].includes(path[0])) return method === "POST";
    return method === "GET";
  }
  return path.length === 3 && path[0] === "sessions" && /^\d+$/.test(path[1]) && path[2] === "revoke" && method === "POST";
}

async function proxy(request: NextRequest, path: string[]) {
  if (!allowed(path, request.method)) return NextResponse.json({ error: "not_found" }, { status: 404 });
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 6000);
  try {
    const headers = new Headers({ Accept: "application/json" });
    const cookie = request.headers.get("cookie");
    const csrf = request.headers.get("x-csrf-token");
    const contentType = request.headers.get("content-type");
    if (cookie) headers.set("Cookie", cookie);
    if (csrf) headers.set("X-CSRF-Token", csrf);
    if (contentType) headers.set("Content-Type", contentType);
    const body = request.method === "GET" ? undefined : await request.text();
    const upstream = await fetch(`${internalBase}/api/account/${path.join("/")}`, {
      method: request.method,
      headers,
      body,
      signal: controller.signal,
      cache: "no-store",
      redirect: "manual",
    });
    const text = await upstream.text();
    const response = new NextResponse(text || null, {
      status: upstream.status,
      headers: {
        "Content-Type": upstream.headers.get("Content-Type") || "application/json; charset=utf-8",
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
    const setCookie = upstream.headers.get("set-cookie");
    if (setCookie) response.headers.set("Set-Cookie", setCookie);
    return response;
  } catch {
    return NextResponse.json({ error: "account_backend_error" }, { status: 502 });
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
