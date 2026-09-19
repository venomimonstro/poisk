# CURRENT SPRINT

**Sprint:** 05 — Extraction + Passages + Dedup
**Status:** IN_PROGRESS

## Goal
Преобразовать ограниченный HTTP body из Sprint 04 в стабильное представление документа для будущей индексации: metadata, canonical, clean text, passages, structured data и duplicate fingerprints без ranking/search logic.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS

## Allowed Work
- HTML parsing with hard input/output limits
- title/meta description/language extraction
- robots meta directives
- canonical URL extraction and validation primitives
- visible clean text extraction
- boilerplate/noise suppression heuristics
- structured data discovery with bounded payloads
- passage splitting with deterministic limits
- exact content hash
- SimHash/near-duplicate fingerprints
- duplicate comparison primitives
- unit/fuzz-style boundary tests

## Forbidden Work
- writing to Manticore index
- ranking/Search API
- query understanding
- Answer Engine
- GEO business logic
- JavaScript/browser rendering
- unbounded DOM or JSON processing

## Definition of Done
- [ ] HTML parser rejects oversized input before expensive processing
- [ ] title/meta description/lang extracted deterministically
- [ ] noindex/nofollow directives represented explicitly
- [ ] canonical URL is parsed but not blindly trusted
- [ ] script/style/noscript/template content excluded from clean text
- [ ] visible text normalized without destroying word boundaries
- [ ] structured-data payload count/size is bounded
- [ ] passages have deterministic max chars/count and stable ordering
- [ ] SHA-256 exact hash produced for normalized content
- [ ] 64-bit SimHash produced for near-duplicate detection
- [ ] Hamming-distance helper covered by tests
- [ ] empty/thin documents handled without panic
- [ ] package tests PASS
- [ ] no GitHub Actions/CI added
- [ ] Sprint 05 report created
