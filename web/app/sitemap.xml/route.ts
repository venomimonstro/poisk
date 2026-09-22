const backend = process.env.API_INTERNAL_BASE_URL || "http://backend:8080";
const shardSize = 500;

function xmlEscape(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&apos;");
}

export async function GET(request: Request) {
  const response = await fetch(`${backend}/api/datahub/sitemap/stats?size=${shardSize}`, {
    headers: { Accept: "application/json" },
    next: { revalidate: 300 },
  });
  if (!response.ok) return new Response("sitemap unavailable\n", { status: 503, headers: { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "no-store" } });
  const stats = (await response.json()) as { shards?: number };
  const shards = Math.max(0, Math.min(100000, Number(stats.shards) || 0));
  const origin = new URL(request.url).origin;
  const body = ["<?xml version=\"1.0\" encoding=\"UTF-8\"?>", "<sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">"];
  for (let shard = 0; shard < shards; shard++) {
    body.push(`<sitemap><loc>${xmlEscape(`${origin}/sitemaps/datahub/${shard}.xml`)}</loc></sitemap>`);
  }
  body.push("</sitemapindex>");
  return new Response(body.join(""), {
    status: 200,
    headers: { "Content-Type": "application/xml; charset=utf-8", "Cache-Control": "public, max-age=300, stale-while-revalidate=600", "X-Content-Type-Options": "nosniff" },
  });
}
