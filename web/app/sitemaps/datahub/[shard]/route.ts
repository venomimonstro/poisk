const backend = process.env.API_INTERNAL_BASE_URL || "http://backend:8080";
const shardSize = 500;

function xmlEscape(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&apos;");
}

export async function GET(request: Request, { params }: { params: Promise<{ shard: string }> }) {
  const { shard } = await params;
  const match = /^(\d+)\.xml$/.exec(shard);
  if (!match) return new Response("not found\n", { status: 404, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });
  const shardNumber = Number(match[1]);
  if (!Number.isSafeInteger(shardNumber) || shardNumber < 0 || shardNumber > 100000) return new Response("not found\n", { status: 404, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });

  const response = await fetch(`${backend}/api/datahub/sitemap/shard?shard=${shardNumber}&size=${shardSize}`, {
    headers: { Accept: "application/json" },
    next: { revalidate: 300 },
  });
  if (!response.ok) return new Response("sitemap unavailable\n", { status: 503, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });
  const body = (await response.json()) as { items?: Array<{ path: string; updated_at: string }> };
  const items = Array.isArray(body.items) ? body.items.slice(0, shardSize) : [];
  if (items.length === 0) return new Response("not found\n", { status: 404, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });
  const origin = new URL(request.url).origin;
  const xml = ["<?xml version=\"1.0\" encoding=\"UTF-8\"?>", "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">"];
  for (const item of items) {
    if (!item.path.startsWith("/data/")) continue;
    const lastmod = new Date(item.updated_at);
    const lastmodText = Number.isNaN(lastmod.getTime()) ? "" : `<lastmod>${lastmod.toISOString()}</lastmod>`;
    xml.push(`<url><loc>${xmlEscape(origin + item.path)}</loc>${lastmodText}</url>`);
  }
  xml.push("</urlset>");
  return new Response(xml.join(""), {
    status: 200,
    headers: { "Content-Type": "application/xml; charset=utf-8", "Cache-Control": "public, max-age=300, stale-while-revalidate=600", "X-Content-Type-Options": "nosniff" },
  });
}
