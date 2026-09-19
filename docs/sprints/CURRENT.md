# CURRENT SPRINT

**Sprint:** 09 — Answer Engine 1.0
**Status:** IN_PROGRESS

## Goal
Добавить source-first Answer Engine поверх Search Alpha: passage/snippet evidence retrieval, source diversity, evidence ranking, extractive claims, citations и confidence gate. Прямой ответ разрешён только когда найдено достаточно проверяемого evidence; иначе система возвращает обычную SERP без выдуманного ответа.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS
- Sprint 05 — PASS
- Sprint 06 — PASS
- Sprint 07 — PASS
- Sprint 08 — PASS (code/static gate; production quality gate requires human judgments)

## Allowed Work
- passage/snippet evidence retrieval from WEB Search
- source clustering/diversity by host
- deterministic evidence scoring
- extractive claim selection
- source/citation model
- confidence calculation and minimum-evidence gate
- Answer API contract
- SERP answer card with cited sources
- tests for unsupported/low-evidence cases

## Forbidden Work
- uncited generated factual claims
- LLM call on every search request
- answers without source URLs
- GEO/organizations/maps
- vector DB/embeddings
- Redis/Kafka/microservices
- JavaScript browser rendering

## Definition of Done
- [ ] evidence candidates come only from Search results
- [ ] evidence text is bounded and HTML-free
- [ ] at least two independent hosts required for Direct Answer
- [ ] no single host can dominate evidence set
- [ ] claims always reference source IDs
- [ ] every source ID resolves to a URL returned in response
- [ ] confidence score deterministic and bounded 0..1
- [ ] low confidence returns fallback without Direct Answer
- [ ] Answer API has stable typed contract
- [ ] SERP shows answer only when confidence gate passes
- [ ] tests cover citation integrity, diversity and fallback
- [ ] no GitHub Actions/CI added
- [ ] Sprint 09 report created
