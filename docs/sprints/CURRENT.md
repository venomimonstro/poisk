# CURRENT SPRINT

**Sprint:** 17 — Growth
**Status:** IN_PROGRESS

## Goal
Добавить безопасные каналы роста поверх Full MVP без изменения поискового ядра: встраиваемый Site Search Widget, интеграции WordPress/1C-Битрикс, Agency flow, organization claiming и referral attribution. Growth-функции должны переиспользовать canonical Search/Webmaster/GEO контуры, а не создавать параллельные индексы.

## Depends On
- Sprint 00–08 — PASS
- Sprint 09–16 — PASS (code/static gate where noted in reports)

## Allowed Work
- publishable domain-bound Site Search Widget keys
- site-scoped Search API using the existing web index
- bounded Origin/host enforcement and widget rate protection
- embeddable JS/CSS widget with accessible fallback
- WordPress plugin for verification, sitemap/URL submit and widget installation
- 1C-Bitrix module for verification, sitemap/URL submit and widget installation
- Agency accounts/workspaces with explicit site delegation
- organization claiming workflow using existing verification/provenance
- referral/campaign attribution for first-party product flows
- Growth analytics that do not alter organic ranking
- tests and operator/admin diagnostics for Growth features

## Forbidden Work
- Sprint 18 billing, subscriptions or paid usage plans
- paid organic ranking or ranking boosts
- separate Elasticsearch/OpenSearch index
- Redis/Kafka/RabbitMQ
- Kubernetes
- arbitrary cross-origin public API access
- secrets embedded in browser/widget code
- automatic organization ownership without verification
- GitHub Actions/CI

## Definition of Done
- [ ] widget publishable keys are site/domain bound and revocable
- [ ] widget Search is scoped to the verified site host in the existing web index
- [ ] widget API has strict query/result/rate/origin bounds
- [ ] embeddable widget works without exposing a secret credential
- [ ] WordPress integration can verify a site and install/configure widget safely
- [ ] Bitrix integration can verify a site and install/configure widget safely
- [ ] Agency flow uses explicit owner delegation and tenant isolation
- [ ] organization claiming requires proof and writes audit/provenance
- [ ] referral attribution is first-party, bounded and does not affect ranking
- [ ] Growth analytics expose usage without storing unnecessary sensitive query data
- [ ] security tests cover key revocation, origin mismatch and tenant delegation
- [ ] no Sprint 18 monetization scope is pulled in
- [ ] no GitHub Actions/CI added
- [ ] Sprint 17 report created
