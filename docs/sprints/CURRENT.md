# CURRENT SPRINT

**Sprint:** 24 — Maps Product + Reviews
**Status:** IN_PROGRESS

## Goal
Сделать публичный Maps продукт на существующем OSM/PMTiles/MapLibre/GEO/address stack и добавить безопасные first-party отзывы организаций, связанные с canonical consumer identity и verified organization claims.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–21 — PASS (code/static gate where noted in reports)
- Sprint 22 — PASS code/static gate; runtime/browser/migration evidence remains external launch gate
- Sprint 23 — PASS code/static gate; runtime/recovery/capacity evidence remains external launch gate

## Allowed Work
- public Yandex-Maps-like layout: search/sidebar/map/card, mobile-first
- GEO/address autocomplete and nearby/category/viewport search using existing canonical services
- organization card with canonical name/category/address/phone/website/hours when evidence exists
- authenticated first-party organization reviews
- one active review per consumer user/place with revision history
- deterministic rating aggregate from visible first-party reviews only
- verified claimed organization owner replies
- report/moderation lifecycle and immutable moderation history
- rate limits / anti-abuse / content length bounds
- safe plain-text review/reply rendering and stored-XSS tests
- tenant/claim isolation tests
- review aggregates must not alter paid or organic ranking without a future explicit ADR
- Admin moderation queue using existing Admin RBAC + CSRF + preview/apply where mutation is critical

## Forbidden Work
- importing third-party review text without explicit source/legal design
- using review rating as organic Search/Maps ranking input
- anonymous review creation
- multiple active ratings by the same consumer for one organization
- owner replies without an active verified organization claim
- raw HTML in reviews/replies
- exposing consumer email/profile identifiers publicly
- Mail implementation (Sprint 25+)
- Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch
- GitHub Actions/CI

## Definition of Done
- [ ] review/revision/report/moderation schema is migration-safe and bounded
- [ ] one active review per consumer/place is enforced by database constraints
- [ ] review edits create immutable revision history
- [ ] visible rating aggregates exclude hidden/deleted/rejected reviews
- [ ] public review API never exposes private consumer identity
- [ ] create/edit/delete/report mutations require consumer session + CSRF
- [ ] owner reply requires verified active organization claim server-side
- [ ] Admin can inspect moderation queue and permissioned actions are audited
- [ ] stored-XSS/IDOR/claim-isolation/rating-recompute tests exist
- [ ] public Maps UI has search/sidebar/map/company card/reviews and responsive mobile layout
- [ ] existing GEO/address/PMTiles stack is reused, not duplicated
- [ ] reviews do not affect organic ranking
- [ ] no Sprint 25+ scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 24 report created
