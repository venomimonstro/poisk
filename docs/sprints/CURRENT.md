# CURRENT SPRINT

**Sprint:** 08 — Search Quality 1.0
**Status:** IN_PROGRESS

## Goal
Сделать качество поиска измеримым и регрессионно контролируемым: golden queries, NDCG/MRR/Recall/ZeroResult, duplicate/spam signals, deterministic rerank baseline и domain diversity без Answer Engine/GEO.

## Depends On
- Sprint 00 — PASS
- Sprint 01 — PASS
- Sprint 02 — PASS
- Sprint 03 — PASS
- Sprint 04 — PASS
- Sprint 05 — PASS
- Sprint 06 — PASS
- Sprint 07 — PASS

## Allowed Work
- golden query dataset/schema/loader
- relevance judgments
- NDCG@10 / MRR / Recall@K / ZeroResult metrics
- Duplicate@10 / domain diversity metrics
- lightweight deterministic reranking
- quality/spam score integration into ranking
- evaluation runner and reports
- regression thresholds
- bounded tests/benchmarks

## Forbidden Work
- Answer Engine
- LLM on every search query
- GEO/organizations/maps
- learned ranking model
- vector DB/embeddings
- Redis/Kafka/microservices

## Definition of Done
- [ ] golden query schema and loader implemented
- [ ] initial golden query seed committed and extensible to 500+
- [ ] NDCG@10 deterministic and tested
- [ ] MRR deterministic and tested
- [ ] Recall@K deterministic and tested
- [ ] ZeroResult and Duplicate@10 measured
- [ ] deterministic rerank incorporates relevance baseline + quality/spam guardrails
- [ ] regression gate supports explicit thresholds
- [ ] evaluation output is machine-readable
- [ ] tests cover metric edge cases
- [ ] no GitHub Actions/CI added
- [ ] Sprint 08 report created
