# CURRENT SPRINT

**Sprint:** 07 — Search Alpha
**Status:** IN_PROGRESS

## Goal
Собрать первый end-to-end WEB Search поверх Manticore: query normalization, русская раскладка/транслитерация/опечатки, bounded search transport, BM25F-oriented field query, snippets, Search API и базовый SERP без Answer Engine/GEO.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS
- Sprint 05 — PASS (code/static gate; no CI added)
- Sprint 06 — PASS (code/static gate; no CI added)

## Allowed Work
- query normalization
- keyboard layout correction
- transliteration variants
- bounded typo/spelling candidates
- Manticore WEB search request/response
- BM25F field weighting baseline
- snippets/highlights
- Search API request validation
- basic SERP frontend integration
- bounded in-process cache
- unit/transport tests

## Forbidden Work
- Answer Engine
- LLM on every search query
- GEO/organizations/maps
- ranking-learning pipeline
- vector database/embeddings
- Redis/Kafka/microservices

## Definition of Done
- [ ] query normalization deterministic
- [ ] RU/EN keyboard-layout correction bounded and tested
- [ ] transliteration variants bounded and tested
- [ ] empty/oversized/abusive query rejected
- [ ] Manticore search request has hard timeout and result limit
- [ ] title/body/description field weights are explicit
- [ ] snippets returned without raw HTML execution
- [ ] domain diversity baseline applied
- [ ] Search API response has stable typed contract
- [ ] repeated query can use bounded local cache
- [ ] transport and query tests added
- [ ] no GitHub Actions/CI added
- [ ] Sprint 07 report created
