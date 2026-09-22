# Sprint 24 — Maps Product + Reviews

**Status:** PASS — code/static gate. Runtime browser, migration and integration evidence remains an external launch gate because this environment does not provide the production database/browser stack and cannot honestly prove runtime execution.

## Delivered

- Reused the existing OSM → PMTiles → MapLibre stack; no second map engine or GEO index was introduced.
- Public `/map` now has organization search/sidebar behavior, viewport organization markers/clusters, organization card and responsive mobile handling.
- Added first-party review core tied to canonical `consumer_users` and canonical organizations.
- Database enforces one review row per `(place_id, consumer_user_id)` and preserves immutable revision history on every state/content/rating change.
- Hard deletion of reviews is blocked; user deletion is a state transition to `DELETED`.
- Rating aggregate is computed only from `VISIBLE` first-party reviews and is recomputed transactionally by database triggers.
- Public review API returns rating/review/reply content without consumer email or consumer user identifiers.
- Review create/edit/delete/report and owner reply require the existing HttpOnly consumer session and CSRF.
- Owner reply is allowed only when the current canonical consumer owns an ACTIVE verified organization claim.
- Claimed owners cannot rate their own organization. If a user review existed before claim activation, the claim trigger automatically changes it to `HIDDEN`, creates a revision and removes it from the rating aggregate.
- Fresh unverified accounts are trust-gated to `PENDING`, so cheap account creation cannot immediately affect public rating. Verified or sufficiently aged accounts follow the normal visible path.
- Account-scoped WRITE/REPORT/REPLY limits are atomic PostgreSQL buckets; retained buckets are pruned every six hours by the existing resource-monitor runtime with seven-day retention.
- Review reports never auto-hide content; Admin moderation is explicit.
- Admin review moderation queue supports HIDE / SHOW / REJECT through RBAC + CSRF + session-bound, expiring, single-use preview/apply.
- Moderation apply verifies the review version/state/place, resolves/dismisses open reports, creates immutable moderation events, creates a new review revision, recomputes rating and writes the general audit log.
- Next review proxy has a strict route/method allowlist and forwards only required Cookie/CSRF/body/query data.
- Review/reply UI uses React text rendering; no raw HTML/dangerouslySetInnerHTML path was added.

## Security / abuse findings fixed during sprint

1. **Owner self-rating after claim** — server rejects reviews from active claimed owners.
2. **Owner self-rating created before claim** — claim activation now suppresses the prior review automatically.
3. **Mass fresh-account rating abuse** — fresh unverified accounts enter `PENDING` and do not affect aggregate rating.
4. **Unbounded action buckets** — maintenance pruning is wired to existing resource-monitor runtime.
5. **IDOR review deletion** — repository mutation requires matching canonical review author.
6. **Stored HTML payload** — JSON output uses HTML-safe encoding and frontend renders text nodes only.

## Tests added

- review edit/revision/rating lifecycle;
- hidden/deleted/pending exclusion from rating;
- hard-delete denial;
- fresh unverified trust gate and edit bypass prevention;
- owner claim reply isolation;
- claimed owner self-review denial;
- pre-claim self-review suppression;
- foreign-user review mutation IDOR denial;
- HTML-safe public JSON output;
- Admin review moderation preview session isolation, single-use apply, report resolution, revision/rating recompute, moderation event and audit evidence.

## Architecture boundaries preserved

- Reviews are not read by WEB/GEO organic ranking code and cannot influence paid placement or organic relevance.
- No third-party review text was imported.
- No anonymous review creation.
- No Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch added.
- No GitHub Actions / CI added.
- Sprint 25 Mail work was not pulled into Sprint 24.

## Runtime evidence still required before commercial launch

- apply migrations on a fresh database and an upgraded prior database;
- run Go unit/integration tests with `TEST_DATABASE_URL`;
- production-like browser smoke test for desktop/mobile Maps and review flows;
- verify resource-monitor pruning and Admin moderation against a live PostgreSQL instance.

These are launch evidence requirements, not claimed as completed by this code/static gate.
