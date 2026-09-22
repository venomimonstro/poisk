import type { Metadata } from "next";
import Link from "next/link";

const backend = process.env.API_INTERNAL_BASE_URL || "http://backend:8080";

type Trend = { day: string; query: string; demand_score: number; gap_score: number; independent_buckets: number };

export const metadata: Metadata = {
  title: "Данные и тренды Poisk",
  description: "Каталоги организаций, сайтов и агрегированные поисковые тренды на основе канонических данных Poisk.",
  alternates: { canonical: "/data" },
  robots: { index: true, follow: true },
};

async function trends(): Promise<Trend[]> {
  const response = await fetch(`${backend}/api/datahub/trends?limit=30`, { headers: { Accept: "application/json" }, next: { revalidate: 300 } });
  if (!response.ok) return [];
  const body = (await response.json()) as { items?: Trend[] };
  return Array.isArray(body.items) ? body.items : [];
}

export default async function DataHubLanding() {
  const items = await trends();
  return (
    <main className="dataHubPage">
      <nav className="dataHubBreadcrumbs" aria-label="Хлебные крошки"><Link href="/">Поиск</Link><span>→</span><span>Данные</span></nav>
      <header className="dataHubHero">
        <div className="dataHubEyebrow">DATA HUB</div>
        <h1>Данные Poisk</h1>
        <p>Каталоги строятся только из канонических организаций, сайтов, адресов и агрегированных сигналов спроса. Персональная история запросов здесь не публикуется.</p>
      </header>
      <section className="dataHubSection">
        <h2>Поисковые тренды</h2>
        {items.length === 0 ? <p>Для публикации трендов пока недостаточно подтверждённых агрегированных сигналов.</p> : (
          <div className="dataHubGrid">
            {items.map((item) => (
              <article className="dataHubCard" key={`${item.day}:${item.query}`}>
                <h3>{item.query}</h3>
                <div className="dataHubCardMeta">Спрос: {item.demand_score}/100 · разрыв покрытия: {item.gap_score}/100 · независимых периодов: {item.independent_buckets}</div>
              </article>
            ))}
          </div>
        )}
      </section>
      <section className="dataHubSection">
        <h2>Что публикуется</h2>
        <p>Города, категории, сочетания город + категория, карточки организаций и сайты появляются только после прохождения минимального порога качества и доказательности. Тонкие и потерявшие актуальность страницы автоматически исключаются из публикации.</p>
      </section>
    </main>
  );
}
