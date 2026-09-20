import { NextRequest, NextResponse } from "next/server";

const internalBase = (process.env.API_INTERNAL_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const maxBodyBytes = 16 * 1024;

async function proxy(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  if (!path.length || path.some(part => !/^[a-zA-Z0-9_-]+$/.test(part))) {
    return NextResponse.json({ error: "invalid_admin_path" }, { status: 400 });
  }

  let body: ArrayBuffer | undefined;
  if (request.method !== "GET" && request.method !== "HEAD") {
    body = await request.arrayBuffer();
    if (body.byteLength > maxBodyBytes) return NextResponse.json({ error: "body_too_large" }, { status: 413 });
  }

  const target = new URL(`${internalBase}/api/admin/${path.join("/")}`);
  request.nextUrl.searchParams.forEach((value, key) => target.searchParams.append(key, value));
  const headers = new Headers({ Accept: "application/json" });
  const cookie = request.headers.get("cookie"); if (cookie) headers.set("cookie", cookie);
  const csrf = request.headers.get("x-csrf-token"); if (csrf) headers.set("x-csrf-token", csrf);
  const contentType = request.headers.get("content-type"); if (contentType) headers.set("content-type", contentType);
  const userAgent = request.headers.get("user-agent"); if (userAgent) headers.set("user-agent", userAgent);

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 5000);
  try {
    const upstream = await fetch(target, { method: request.method, headers, body, signal: controller.signal, cache: "no-store", redirect: "manual" });
    const payload = await upstream.arrayBuffer();
    const response = new NextResponse(payload, { status: upstream.status });
    response.headers.set("Content-Type", upstream.headers.get("Content-Type") || "application/json; charset=utf-8");
    response.headers.set("Cache-Control", "no-store");
    response.headers.set("X-Content-Type-Options", "nosniff");
    const setCookie = upstream.headers.get("set-cookie"); if (setCookie) response.headers.set("set-cookie", setCookie);
    return response;
  } catch {
    return NextResponse.json({ error: "admin_backend_error" }, { status: 502 });
  } finally {
    clearTimeout(timer);
  }
}

export const GET = proxy;
export const POST = proxy;
