import { NextRequest, NextResponse } from "next/server";
import { ProxyBodyTooLarge, readLimitedProxyBody } from "../../_proxyBody";

const internalBase = (process.env.API_INTERNAL_BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const maxBodyBytes = 16 * 1024;

const getRoutes = new Set([
  "me","csrf","status","domains","diagnostics","mail/gateway-health","mail/dead-letters",
  "query-gaps","users","webmaster/sites","billing/invoices","datahub/pages","recovery",
  "quality/latest","answer/metrics","organizations/imports","organizations/reviews",
  "reviews/moderation","maps/state","addresses/data","users/sessions","users/security-events","owner",
]);
const postRoutes = new Set([
  "login","logout","domains/preview","domains/apply","mail/dead-letters/retry/preview","mail/dead-letters/retry/apply",
  "query-gaps/preview","query-gaps/apply","datahub/preview","datahub/apply",
  "organizations/reviews/preview","organizations/reviews/apply","reviews/moderation/preview","reviews/moderation/apply",
]);

function allowed(path: string[], method: string) {
  if (!path.length || path.some(part => !/^[a-zA-Z0-9_-]+$/.test(part))) return false;
  const key = path.join("/");
  if (method === "GET") return getRoutes.has(key);
  if (method === "POST") return postRoutes.has(key);
  return false;
}

async function proxy(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  if (!allowed(path, request.method)) {
    return NextResponse.json({ error: "admin_route_not_exposed" }, { status: 404 });
  }

  let body: Uint8Array | undefined;
  try {
    if (request.method !== "GET" && request.method !== "HEAD") body = await readLimitedProxyBody(request, maxBodyBytes);
  } catch (error) {
    if (error instanceof ProxyBodyTooLarge) return NextResponse.json({ error: "body_too_large" }, { status: 413 });
    return NextResponse.json({ error: "invalid_body" }, { status: 400 });
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
