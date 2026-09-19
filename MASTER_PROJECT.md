# Независимая поисковая система РФ
## MASTER PROJECT — улучшенное описание проекта и детальное техническое задание

**Статус:** Ready for Development  
**Назначение:** владелец проекта, CTO, команда разработки, AI-разработчик, инвесторы  
**Подход:** бюджетный production-minded startup  
**Ключевая цель:** создать быстрый поисковый продукт уровня современного Search + Answer Engine без инфраструктуры уровня Яндекса/Google

---

# 1. Концепция проекта

Проект — независимая поисковая платформа по российскому интернету, которая объединяет:

1. **Web Search** — классическая поисковая выдача.
2. **Answer Engine** — готовый ответ на вопрос пользователя с источниками.
3. **GEO / Карты** — поиск организаций, адресов и локальных услуг.
4. **Webmaster** — добавление сайтов, индексация, диагностика и аналитика.
5. **Site Search** — поиск по сторонним сайтам как B2B-продукт.
6. **Search API / GEO API** — программный доступ для бизнеса.
7. **Admin** — управление crawler, индексом, качеством, безопасностью и GEO.

Позиционирование:

> **Поиск, который не заставляет искать ответ.**

Пользователь должен получать:

```text
готовый ответ
+
источники
+
обычную выдачу сайтов
+
организации и карту, если запрос локальный
```

Проект не строится как «маленький Яндекс». Его преимущество — управляемая и дешёвая архитектура:

```text
контролируемый индекс
+
Demand Driven Crawling
+
русскоязычный Query Understanding
+
Answer Engine
+
Web + GEO
+
Webmaster supply loop
+
низкая стоимость одного запроса
```

---

# 2. Основная ценность для пользователя

Классический поиск заставляет пользователя:

```text
ввести запрос
→ открыть несколько сайтов
→ найти нужный абзац
→ сравнить информацию
→ сформировать ответ самостоятельно
```

Наш продукт:

```text
вопрос
→ поиск
→ проверка нескольких источников
→ готовый ответ
→ ссылки на доказательства
```

---

# 3. Отличие от конкурентов

## 3.1 Answer-first Search
Пользователь получает сначала решение задачи, а не только список URL.

## 3.2 Source-first Answers
Ответ строится только из найденных источников. Каждое важное утверждение должно иметь ссылку на доказательство.

## 3.3 Demand Driven Index
Crawler не пытается индексировать интернет целиком. Если спрос высокий, а выдача слабая, создаётся Query Gap и повышается crawl priority.

## 3.4 Controlled Corpus
Система индексирует прежде всего полезные и востребованные документы.

## 3.5 Web + GEO
Один Query Planner понимает WEB / ANSWER / GEO / ADDRESS / NAVIGATIONAL / MAP.

## 3.6 Webmaster Loop
Вебмастеры сами добавляют сайты и сообщают об изменениях страниц.

## 3.7 Cost-first Architecture
Каждое улучшение должно проходить правило: Quality ↑ AND Cost/query remains acceptable.

---

# 4. Пользовательские продукты

## 4.1 Web Search
- русский полнотекстовый поиск;
- словоформы;
- опечатки;
- неправильная раскладка;
- транслитерация;
- бренды и модели;
- фразы;
- GEO;
- snippets;
- domain diversity;
- anti-spam;
- ranking;
- быстрый fallback.

## 4.2 Answer Engine
Факт, определение, инструкция, сравнение, краткая сводка, локальный вопрос, вопрос по товару/услуге.

## 4.3 Deep Answer
Отдельный режим «Исследовать подробнее». Может выполнять subqueries, анализировать больше источников и формировать подробный результат. Не блокирует обычный Search.

## 4.4 GEO
Организации, категории, районы, города, nearby, карта, адреса, карточки.

## 4.5 Webmaster
Сайт, sitemap, URL Submit/Delete, reindex, диагностика, показы, клики, CTR, Answer citations.

---

# 5. Монетизация

- Webmaster Pro: ориентир 1 490 ₽/мес.
- Agency: 4 990–9 990 ₽/мес.
- Site Search Pro: 1 490–4 990 ₽/мес.
- Business Pro: 990–1 990 ₽/мес.
- Search / GEO API: usage-based или подписка.
- Реклама только после значительного consumer-трафика.
- Organic ranking не продаётся.

---

# 6. Техническая философия

> **В момент пользовательского запроса нельзя делать тяжёлую работу, если её можно выполнить заранее.**

Offline:

```text
Crawler → Fetch → Clean → Passages → Entities → Dedup → Quality → Spam → Index
```

Online:

```text
Query → Normalize → Intent → Cache → Search → TOP candidates → lightweight rerank → Answer → Response
```

---

# 7. Зафиксированный технологический стек

## Backend
Go, Chi Router, pgx/v5, sqlc, net/http, log/slog.

Правила: ORM запрещён; SQL хранится явно; sqlc генерирует типизированный доступ; один modular monolith; микросервисы запрещены до отдельного ADR.

## Frontend
Next.js, TypeScript, MapLibre GL JS.

## Search
Manticore Search, RT tables, BM25/BM25F, Russian morphology, stored fields.

## Database
PostgreSQL, PostGIS.

## Maps
OpenStreetMap → PMTiles/vector tiles → MapLibre.

## Deployment
Linux VPS, Docker Compose, Nginx, versioned Docker images.

---

# 8. Запрещённые технологии на MVP

Без ADR запрещено добавлять Elasticsearch/OpenSearch, Kubernetes, Kafka, RabbitMQ, Redis, vector DB, собственный inverted index, собственный HOT/COLD manager, embeddings всего Web, LLM на каждый Search query, обязательную GPU-инфраструктуру и микросервисы.

---

# 9. Архитектура Search

```text
USER
 ↓
CDN/WAF
 ↓
NGINX
 ↓
NEXT.JS
 ↓
SEARCH API
 ↓
QUERY UNDERSTANDING
 ↓
QUERY PLANNER
 ├─ WEB → Manticore
 ├─ GEO → Manticore/PostGIS
 └─ ADDRESS → Address Index
 ↓
CANDIDATES
 ↓
RERANK
 ↓
PASSAGE ENGINE
 ↓
ANSWER ENGINE
 ↓
CONFIDENCE GATE
 ↓
ANSWER + SERP
```

---

# 10. Backend modules

```text
/internal
  /search
  /query
  /answer
  /crawler
  /scheduler
  /fetcher
  /extractor
  /dedup
  /indexer
  /webmaster
  /organizations
  /geo
  /admin
  /auth
  /billing
  /analytics
  /platform
```

Режимы: `app api`, `app crawler`, `app scheduler`, `app indexer`, `app worker`, `app maintenance`.

---

# 11. Источник истины

PostgreSQL хранит канонические данные: domains, urls, document_versions, crawl_queue, crawl_history, index_outbox, users, sites, organizations, webmaster, billing, audit_log, system_settings.

Manticore хранит searchable representation и должен быть rebuildable, replaceable, versioned.

---

# 12. Transactional Outbox

В одной транзакции PostgreSQL выполняются canonical update + запись index_outbox. После COMMIT Indexer выполняет idempotent REPLACE/DELETE в Manticore. Каждая индексируемая сущность имеет `version BIGINT`, старая версия не может перезаписать новую.

---

# 13. Crawler Architecture

Источники URL: trusted seeds, категории сайтов, Webmaster, organization websites, sitemap, Domain Graph, Query Gap.

Основной discovery: `DOMAIN → robots.txt → sitemap → crawl queue`. Link discovery применяется ограниченно.

---

# 14. Crawl Budget

Для каждого домена: crawl_budget, max_urls, max_depth, requests_per_second, max_concurrency, max_bytes_per_day, next_crawl_at.

Новый сайт: 100–500 URL. Рост до 5k → 20k → 50k+ только при хорошем качестве.

---

# 15. Scheduler

Global Scheduler выбирает домен. Domain Scheduler выбирает URL.

```text
priority = demand × expected_quality × freshness_need × change_probability ÷ fetch_cost
```

---

# 16. Crawl Queue

Поля: id, url_id, domain_id, priority, status, attempts, available_at, lease_until, worker_id, last_error. Workers используют `FOR UPDATE SKIP LOCKED`. Истёкший lease возвращает задачу в READY.

---

# 17. Защита crawler

Перед каждым соединением: parse URL → validate scheme → resolve DNS → validate IP → connect. Запрещены localhost, private ranges, link-local, metadata IP и internal DB/admin addresses. После redirect проверка повторяется.

---

# 18. Crawler Limits

Обязательны max response bytes, max decompressed bytes, max redirects, max sitemap bytes, max sitemap depth, max URLs/day/domain, fetch timeout, content-type allowlist.

---

# 19. URL Trap Detection

Определять calendars, session IDs, search pages, endless filters, sort permutations, pagination loops, parameter explosion. При anomaly: domain → LIMITED, crawl budget ↓.

---

# 20. HTTP Fetcher

Concurrent HTTP, connection pooling, keep-alive, retry/backoff, ETag, Last-Modified, conditional requests, redirect/response limits. JS rendering OFF by default.

---

# 21. Content Extraction

Извлекать title, description, H1, H2/H3, main_text, canonical, language, published_at, updated_at, links, structured_data. Удалять navigation/footer/scripts/styles/cookie blocks/advertising boilerplate/template repetition.

---

# 22. Passage Engine

Страница разбивается на 5–15 смысловых блоков: passage_id, doc_id, heading, text, position, entities, quality. Passages не являются отдельными Web documents.

---

# 23. Dedup

URL normalize → canonical → exact content hash → SimHash → duplicate cluster. Exact duplicates можно схлопывать; near duplicates получают cluster + ranking penalty и не удаляются автоматически только по SimHash.

---

# 24. Search Schema

Web: doc_id, version, domain_id, url, title, description, h1, body, language_id, region_id, published_at, updated_at, quality_score, spam_score, domain_score.

Organizations: place_id, version, name, category_id, city_id, district_id, address, text, lat, lon, rating, quality_score.

---

# 25. Query Understanding

RAW QUERY → Unicode normalize → tokenize → mixed-script check → keyboard layout → spelling → transliteration → exact tokens → lemmas → phrase detection → brand/product entity → GEO entity → intent → Query Plan. Исходный query сохраняется всегда.

---

# 26. Query Planner

Routes: WEB, ANSWER, GEO, ADDRESS, NAVIGATIONAL, WEB+GEO, MAP+WEB.

---

# 27. Ranking 1.0

Без ML: BM25F + exact match + title boost + H1 boost + phrase match + quality + freshness + domain authority + geo relevance - spam. Retrieval TOP 200–500. Rerank CPU-only.

---

# 28. Diversity

Для обычного query не более 2–3 URL/domain в TOP-10; исключение — navigational intent.

---

# 29. Answer Engine

Query → TOP documents → Source Diversity → Passage Retrieval → Evidence Ranking → Source Clustering → Claim Extraction → Consensus/Conflict → Answer Composer → Citation Validator → Confidence Gate.

---

# 30. Answer Engine 1.0

Без большой генеративной LLM. Типы: Fact, Definition, How-to, Comparison, Local, Short Summary. При недостаточном confidence показывается SERP only.

---

# 31. Confidence

Базово: 80–100 Direct Answer; 55–79 cautious answer; 0–54 suppress answer. Для медицины, права, финансов и безопасности порог выше.

---

# 32. Source Independence

Строится source_cluster по similarity, publication dates, citations и domain relationships. Копии одного материала не считаются независимыми источниками.

---

# 33. Fast / Deep Answer

Fast — короткий ответ в основной выдаче. Deep — отдельная команда «Исследовать подробнее» с несколькими subqueries и большим числом sources.

---

# 34. Progressive Rendering

SERP ready → render; Answer ready → append; GEO ready → append. Самый медленный компонент не блокирует выдачу.

---

# 35. Performance

Цели базового Search: P50 <150 ms, P95 <300 ms, P99 <700 ms. Fallback: Answer timeout → SERP; Reranker timeout → BM25; GEO timeout → WEB.

---

# 36. Cache

На MVP in-process TinyLFU/LRU. Key: normalized_query + region + filters + ranking_version + index_version. Single-flight, TTL jitter, stale-while-revalidate.

---

# 37. Load Shedding

NORMAL: Search + Answer + GEO. HIGH: Search + cached Answer. CRITICAL: Search + BM25. ATTACK: cached/basic Search + strict rate limits.

---

# 38. GEO Architecture

SOURCE ADAPTERS (2GIS, OSM, websites, owner data, future sources) → RAW → STAGING → NORMALIZATION → OUR PLACE. Production model не зависит от структуры одного источника.

---

# 39. OUR PLACE

place_id, version, brand_id, name, category_id, lat, lon, address_id, phone, website, opening_hours, source, source_id, source_rating, review_count, confidence, status, verified_owner_id, last_verified_at.

---

# 40. GEO Dedup

Использовать name similarity, phone, domain, address, coordinates, category. High confidence → auto merge; medium → review. Филиалы одного бренда не объединять автоматически.

---

# 41. Maps

OSM → versioned PMTiles → MapLibre. Организации: viewport filtering, clustering, zoom-dependent density. Не отправлять все organizations одним GeoJSON.

---

# 42. Webmaster

Free: add site, verify, sitemap, URL Submit/Delete, reindex, index status, crawl errors, robots, canonical, 404/500, impressions, clicks, CTR, Answer citations.

Pro: multiple sites, scheduled audits, history, alerts, expanded reports, API.

---

# 43. Admin

Dashboard, Crawler, Domains, URLs, Index, Search Quality, Queries, Answer Engine, Spam, Organizations, Maps, Webmaster, Users, Billing, System. Обязательны RBAC, 2FA, audit log, dry-run, rollback, mass-action preview.

---

# 44. Security

Auth: Argon2id, HttpOnly cookies, Secure, SameSite, CSRF, login throttling. API: лимиты query bytes, token count, wildcards, result depth, max results, query timeout, QPS, concurrency. Пользователь не имеет прямого доступа к Manticore/PostgreSQL.

---

# 45. Backup / Recovery

Backup PostgreSQL, configs, organizations, Webmaster, billing, dictionaries. Search index rebuildable. После роста добавить Recovery Archive. Restore test обязателен.

---

# 46. Versioned Index

web_v1 → build web_v2 → quality test → benchmark → switch. Rollback web_v2 → web_v1.

---

# 47. MVP Infrastructure

Ориентир: 8–16 vCPU, 16–32 GB RAM, 500 GB–1 TB NVMe. Первый server: Nginx, Frontend, API, Manticore, PostgreSQL/PostGIS, Crawler. Crawler ограничен CPU/RAM/I/O.

---

# 48. Масштабирование

Stage 1: 1 server. Stage 2: Search+DB+API отдельно, Crawler+Extractor+Indexer отдельно. Stage 3: Search API → Distributed Manticore → shards. Любое масштабирование только после benchmark и ADR.

---

# 49. Search Quality Lab

500 → 1000 → 5000 golden queries. Метрики: NDCG@10, MRR, Recall@100, ZeroResult, Spam@10, Duplicate@10, Answer Accuracy, Citation Precision, P50/P95/P99.

---

# 50. Search Coverage Score

По категориям: Demand, Coverage, Quality, Freshness, Spam. Если Demand HIGH и Coverage/Quality LOW, создаётся QUERY GAP.

---

# 51. Demand Driven Index

Query Gap влияет на crawl priority, seed discovery, category budget и recrawl frequency.

---

# 52. Quality / Cost Gate

Новая Search-функция разрешается только если Quality improves и Cost/query remains within budget.

---

# 53. AI Development Protocol

Главный источник истины: `MASTER_PROJECT.md`. Перед работой AI читает `MASTER_PROJECT.md`, `docs/adr/*`, `docs/sprints/CURRENT.md`. Разрешён только один активный sprint. Запрещено менять stack/architecture, добавлять infrastructure component, переходить к следующему sprint до DoD, делать будущий функционал «заодно», отключать tests и скрывать P0/P1 bug.

---

# 54. Roadmap

Ориентир 2 недели/sprint, но переход только после DoD.

## Sprint 00 — Project Contract
Repository, MASTER_PROJECT, ADR, sprint template, Makefile, coding rules, config, logging, version policy, Definition of Done.

## Sprint 01 — Runtime Foundation
Docker Compose, Go backend, Next.js, PostgreSQL/PostGIS, Manticore, migrations, typed config, health endpoints, structured logs. DoD: stack starts with one command.

## Sprint 02 — Canonical Data + Queue
Domains, URLs, document_versions, crawl_queue, leases, versions, index_outbox, audit log. Tests: worker crash, duplicate job, lease expiry, stale version.

## Sprint 03 — Scheduler + Crawler Security
Crawl budgets, global/domain scheduler, robots, sitemap, SSRF protection, sitemap limits, URL traps.

## Sprint 04 — HTTP Fetcher
HTTP pool, redirects, timeout, retries/backoff, ETag, Last-Modified, response limits. DoD: long-running crawl without memory/goroutine leak.

## Sprint 05 — Extraction + Passages + Dedup
Content extraction, metadata, passages, hashes, SimHash, canonical, structured data.

## Sprint 06 — Manticore Indexing
Web schema, outbox indexer, entity versions, idempotent updates, rebuild command.

## Sprint 07 — Search Alpha
Query Understanding, morphology, spelling, keyboard layout, transliteration, BM25F, snippets, Search API, basic SERP, cache.

## Sprint 08 — Search Quality 1.0
500 golden queries, Quality Score, Spam Score, domain graph, Ranking 1.0, diversity, regression metrics.

## Sprint 09 — Answer Engine 1.0
Passage retrieval, source diversity/clustering, evidence ranking, claim extraction, citations, confidence, fallback. DoD: любой Direct Answer имеет проверяемые sources.

## Sprint 10 — Performance / Load Protection
Progressive rendering, deadlines, load shedding, single-flight, rate limits, query complexity limits, benchmark.

## Sprint 11 — Webmaster Free
Auth, ownership verification, sitemap, URL Submit/Delete, index status, diagnostics, impressions/clicks, Answer citations.

### Gate A — Public Search Alpha
Web Search + Answer Engine + Webmaster Free.

## Sprint 12 — Maps
OSM, PMTiles, MapLibre, map versioning, rollback.

## Sprint 13 — Organizations Import
Source adapters, staging, validation, normalization, OUR PLACE, dedup, dry-run, resumable import.

## Sprint 14 — GEO Search
Organizations index, PostGIS, city/category, nearby, cards, map clustering, GEO Query Planner.

## Sprint 15 — Address + Web ↔ GEO
ФИАС/ГАР, address index, autocomplete, geocoding, Schema.org, domain/place matching.

## Sprint 16 — Hardening
Admin, RBAC, 2FA, backup, restore, disk/memory protection, observability, release, rollback, security tests.

### Gate B — Full MVP
Search + Answer Engine + Webmaster + Maps + Organizations + GEO + Admin + Backup/Restore.

## Sprint 17 — Growth
WordPress plugin, Bitrix module, Site Search Widget, Agency flow, organization claiming, referral tracking.

## Sprint 18 — Monetization
Webmaster Pro, Agency, Site Search Pro, Business Pro, billing, usage limits.

## Sprint 19 — Demand Driven Index
Query Gap, Search Coverage Score, demand score, crawler feedback, manipulation protection.

## Sprint 20 — Data Hub
City/category pages, organization/website directories, trends, first-party data, programmatic SEO, AI summaries only as auxiliary content.

## Sprint 21 — Capacity Gate
Benchmark 1M docs and target 10M model: QPS, P50/P95/P99, CPU, RAM, disk, crawl/index throughput, outbox backlog. После benchmark ADR: stay single node / move crawler / shard search / add replica.

---

# 55. Definition of Done

Sprint закрывается только если: code builds; unit/integration tests PASS; migrations PASS; previous DB upgrade PASS; health checks PASS; security tests PASS; нет P0/P1 bugs; документация обновлена; rollback/recovery описан; sprint report создан.

---

# 56. Критические риски

| Риск | Решение |
|---|---|
| PostgreSQL/Manticore desync | transactional outbox |
| worker crash | lease + idempotency |
| duplicate jobs | unique active task |
| SSRF | DNS/IP validation + network isolation |
| sitemap bomb | strict limits |
| URL explosion | pattern detection + budgets |
| duplicate explosion | hash + SimHash |
| canonical poisoning | confidence rules |
| query DoS | planner + hard limits |
| ranking regression | Quality Lab |
| Answer hallucination | evidence + confidence + citations |
| source copying | source clustering |
| disk full | watermarks |
| bad release | version + rollback |
| GEO corruption | staging + dry-run |
| AI scope creep | MASTER + ADR + sprint gates |

---

# 57. Цель первой версии

Не 100 млн страниц, а быстрый Web Search + готовый Answer + источники + Webmaster + измеримое качество + реальные пользователи + контролируемая стоимость.

---

# 58. Критерий жизнеспособности

Проект жизнеспособен, если TOP-10 достаточно качественный; Answer Engine экономит время пользователя; Citation Precision высокая; пользователи возвращаются; Webmaster приводит новые сайты; crawler остаётся дешёвым; P95 остаётся в бюджете; инфраструктура масштабируется предсказуемо; появляются B2B-платежи.

```text
Пользовательский спрос
→ качественный индекс
→ быстрый Search
→ готовый Answer
→ outbound traffic
→ Webmaster
→ больше качественных данных
→ лучше Search
→ B2B revenue
→ controlled scaling
```
