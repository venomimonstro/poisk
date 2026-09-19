# Sprint 09 — Answer Engine 1.0

**Status:** PASS (code/static gate)

## Implemented
- source-first extractive Answer Engine over existing WEB Search results;
- evidence can originate only from returned Search snippets/results;
- HTML markers/tags are removed and evidence length is hard-bounded;
- maximum one evidence candidate per host and maximum four sources;
- Direct Answer requires at least two independent hosts;
- deterministic evidence relevance and confidence scoring, bounded to 0..1;
- low-confidence and insufficient-source paths return fallback without a Direct Answer;
- answer text is extractive: no factual connective prose is synthesized;
- every claim contains source IDs and every source ID maps to a URL in the response;
- stable `/api/answer?q=...` JSON contract with query/backend error separation;
- Answer Engine reuses the same cached Search service instead of adding another retrieval infrastructure path;
- same-origin Next.js Answer API proxy for local frontend development;
- responsive SERP answer card with clickable citations;
- answer/evidence content is rendered as React text, never raw HTML;
- stale Answer responses from an earlier query cannot overwrite a newer SERP.

## Tests added
- independent-host requirement;
- citation/source integrity;
- bounded confidence and low-confidence fallback;
- HTML-free bounded evidence;
- Answer HTTP response and query/backend error mapping.

## Important behavior
If evidence is weak, duplicated by host, too short, or below the confidence threshold, the system shows the ordinary SERP only. Sprint 09 deliberately does not introduce an LLM in the request path.

## Verification
The implementation and regression tests were statically reviewed against Sprint 09 DoD. The current execution environment still cannot run a fresh dependency-backed repository-wide Go/Next build without external dependency access; this report does not claim a CI run. GitHub Actions/CI were not added.

## Rollback
Sprint 09 is isolated to `internal/answer`, `/api/answer` wiring, the local Next.js proxy, and the answer-card UI. Removing those paths leaves Search Alpha intact.
