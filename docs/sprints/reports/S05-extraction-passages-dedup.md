# Sprint 05 — Extraction + Passages + Dedup

**Status:** PASS (code/static gate)

## Implemented
- bounded HTML input and DOM node processing;
- deterministic title, description and lang extraction;
- canonical parsing with explicit validity and same-host trust signal;
- meta robots and `X-Robots-Tag` handling, including scoped `PoiskBot` directives;
- removal of script/style/noscript/template and common navigation/form noise from clean text;
- hard clean-text output limit;
- bounded JSON-LD collection;
- deterministic UTF-8 safe passage splitting;
- SHA-256 exact content hash;
- 64-bit SimHash, Hamming distance and near-duplicate predicate;
- boundary/regression tests for HTML limits, passages, hashes and robots directives.

## Important fixes found during sprint
- final clean-text hard cap now includes separators between text nodes;
- scoped `X-Robots-Tag` state no longer leaks directives from another crawler to PoiskBot;
- canonical is represented as a signal and is not blindly trusted.

## Verification
Repository code and tests were reviewed against the Sprint 05 DoD. The execution environment used for this session has no direct GitHub/DNS access, so a fresh dependency-backed `go test ./...` could not be executed locally without introducing CI. Per project instruction, GitHub Actions/CI was not added.

## Rollback
All Sprint 05 work is isolated in `internal/extractor` plus this report/current-sprint metadata. Rollback can revert Sprint 05 commits without touching crawler queue or fetcher persistence.
