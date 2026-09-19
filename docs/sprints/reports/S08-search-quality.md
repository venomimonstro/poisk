# Sprint 08 — Search Quality 1.0

**Status:** PASS (code/static gate)

## Implemented
- deterministic NDCG@10, MRR, Recall@10, ZeroResult and Duplicate@10 metrics;
- versioned golden-query schema with normalized URL judgments and grade 0..3;
- initial 30-query seed spanning informational, navigational, commercial, layout, transliteration, long, freshness, comparison and mixed-language cases;
- bounded golden/threshold loaders with strict JSON decoding;
- machine-readable per-query and aggregate evaluator;
- regression gate with explicit thresholds;
- minimum judged-query threshold (50 by default) so tiny hand-picked samples cannot pass the gate;
- deterministic lexical rerank guardrails using indexed `quality_score` and `spam_score`;
- quality signal influence is bounded: quality can lift lexical score by at most 20%, severe spam can demote to 20%;
- `app quality` manual runtime reads golden + thresholds, performs real searches, prints JSON and exits with failure when gate fails;
- quality datasets are included in the backend container image.

## Important behavior
The committed seed intentionally contains unjudged queries. Unjudged queries are skipped by evaluation instead of being treated as zero-relevance. The gate cannot pass until enough human judgments exist.

## Regression thresholds
Current committed thresholds require at least 50 judged queries plus minimum NDCG/MRR/Recall and maximum ZeroResult/Duplicate rates. Thresholds are data, not hidden constants, and can be reviewed/versioned.

## Verification
Metric/golden/config/evaluator/gate/rerank regression tests are present. Repository-wide dependency-backed test execution is still unavailable in the current isolated runtime; this report therefore records a code/static gate and does not claim a CI run. GitHub Actions/CI were not added.

## Rollback
Quality evaluation is isolated under `internal/quality`, `docs/quality`, and the bounded rerank in `internal/search`. The quality runtime is optional and does not affect API startup.
