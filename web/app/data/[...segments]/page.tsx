import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

type HubPage = {
  page_id: number;
  page_type: "CITY" | "CATEGORY" | "CITY_CATEGORY" | "ORGANIZATION" | "WEBSITE";
  slug: string;
  canonical_path: string;
  title: string;
  meta_description: string;
  version: number;
  evidence_count: number;
  quality_score: number;
  refreshed_at?: string;
  snapshot: Record<string, unknown>;
  related: Array<{ path: string; title: string; page_type: string }>;
};

type Organization = {
  place_id: number;
  name: string;
  city_key?: string;
  category_key?: string;
  address?: string;
  website?: string;
  quality_score: number;
};

const backend = process.env.API_INTERNAL_BASE_URL || "http://backend:8080";

async function fetchPage(path: string): Promise<HubPage | null> {
  const response = await fetch(`${backend}/api/datahub/page?path=${encodeURIComponent(path)}`, {
    headers: { Accept: "application/json" },
    next: { revalidate: 60 },
  });
  if (response.status === 404) return null;
  if (!response.ok) throw new Error(`datahub_page_${response.status}`);
  const body = (await response.json()) as { page: HubPage };
  return body.page;
}

function dataPath(segments: string[]): string {
  if (!segments.length || segments.length > 4) return "";
  if (segments.some((segment) => !segment || segment.length > 180 || segment.includes("/") || segment.includes("\\") || segment === "." || segment === "..")) return "";
  return `/data/${segments.join("/")}`;
}

function safeExternalURL(value: string): string {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.toString() : "";
  } catch {
    return "";
  }
}

function snapshotString(snapshot: Record<string, unknown>, key: string): string {
  const value = snapshot[key];
  return typeof value === "string" ? value : "";
}

function snapshotNumber(snapshot: Record<string, unknown>, key: string): number {
  const value = snapshot[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

async function fetchOrganizations(page: HubPage): Promise<Organization[]> {
  if (!(["CITY", "CATEGORY", "CITY_CATEGORY"] as string[]).includes(page.page_type)) return [];
  const aggregate = page.snapshot.aggregate;
  const a = typeof aggregate === "object" && aggregate !== null ? aggregate as Record<string, unknown> : {};
  const city = typeof a.city_key === "string" ? a.city_key : "";
  const category = typeof a.category_key === "string" ? a.category_key : "";
  const params = new URLSearchParams({ limit: "30" });
  if (city) params.set("city", city);
  if (category) params.set("category", category);
  const response = await fetch(`${backend}/api/datahub/organizations?${params.toString()}`, {
    headers: { Accept: "application/json" },
    next: { revalidate: 60 },
  });
  if (!response.ok) return [];
  const body = (await response.json()) as { items?: Organization[] };
  return Array.isArray(body.items) ? body.items : [];
}

export async function generateMetadata({ params }: { params: Promise<{ segments: string[] }> }): Promise<Metadata> {
  const { segments } = await params;
  const path = dataPath(segments);
  if (!path) return { robots: { index: false, follow: false } };
  const page = await fetchPage(path);
  if (!page) return { robots: { index: false, follow: false } };
  return {
    title: page.title,
    description: page.meta_description,
    alternates: { canonical: page.canonical_path },
    robots: { index: true, follow: true },
  };
}

export default async function DataHubPage({ params }: { params: Promise<{ segments: string[] }> }) {
  const { segments } = await params;
  const path = dataPath(segments);
  if (!path) notFound();
  const page = await fetchPage(path);
  if (!page) notFound();
  const organizations = await fetchOrganizations(page);
  const organizationName = snapshotString(page.snapshot, "name");
  const organizationAddress = snapshotString(page.snapshot, "address");
  const organizationWebsite = safeExternalURL(snapshotString(page.snapshot, "website"));
  const websiteHost = snapshotString(page.snapshot, "host");
  const linkedOrganizations = snapshotNumber(page.snapshot, "linked_organizations");
  const indexedURLs = snapshotNumber(page.snapshot, "indexed_urls");

  const breadcrumbJson = {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: [
      { "@type": "ListItem", position: 1, name: "Поиск", item: "/" },
      { "@type": "ListItem", position: 2, name: "Данные", item: "/data" },
      { "@type": "ListItem", position: 3, name: page.title, item: page.canonical_path },
    ],
  };

  return (
    <main className="dataHubPage">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(breadcrumbJson).replace(/</g, "\\u003c") }} />
      <nav className="dataHubBreadcrumbs" aria-label="Хлебные крошки">
        <Link href="/">Поиск</Link><span>→</span><span>{page.title}</span>
      </nav>
      <header className="dataHubHero">
        <div className="dataHubEyebrow">Данные Poisk · {page.page_type}</div>
        <h1>{page.title}</h1>
        <p>{page.meta_description}</p>
        <div className="dataHubStats">
          <span>Источников/объектов: {page.evidence_count}</span>
          <span>Качество данных: {page.quality_score}/100</span>
          {page.refreshed_at && <span>Обновлено: {new Date(page.refreshed_at).toLocaleDateString("ru-RU")}</span>}
        </div>
      </header>

      {organizations.length > 0 && (
        <section className="dataHubSection">
          <h2>Организации</h2>
          <div className="dataHubGrid">
            {organizations.map((item) => {
              const website = item.website ? safeExternalURL(item.website) : "";
              return (
                <article className="dataHubCard" key={item.place_id}>
                  <h3>{item.name}</h3>
                  {item.address && <p>{item.address}</p>}
                  <div className="dataHubCardMeta">Качество: {Math.round(item.quality_score)}/100</div>
                  <div className="dataHubCardLinks">
                    <Link href={`/data/organization/org-${item.place_id}`}>Карточка</Link>
                    {website && <a href={website} rel="nofollow noopener noreferrer">Сайт</a>}
                  </div>
                </article>
              );
            })}
          </div>
        </section>
      )}

      {page.page_type === "ORGANIZATION" && (
        <section className="dataHubSection">
          <h2>{organizationName || "Организация"}</h2>
          {organizationAddress && <p>{organizationAddress}</p>}
          {organizationWebsite && <p><a href={organizationWebsite} rel="nofollow noopener noreferrer">Официальный сайт</a></p>}
        </section>
      )}

      {page.page_type === "WEBSITE" && (
        <section className="dataHubSection">
          <h2>{websiteHost || page.title}</h2>
          <p>Связанных организаций: {linkedOrganizations}. Индексированных страниц: {indexedURLs}.</p>
        </section>
      )}

      {page.related.length > 0 && (
        <section className="dataHubSection">
          <h2>Связанные страницы</h2>
          <ul className="dataHubRelated">
            {page.related.map((item) => <li key={item.path}><Link href={item.path}>{item.title}</Link></li>)}
          </ul>
        </section>
      )}
    </main>
  );
}
