# Sprint 17 — Growth Report

**Status:** PASS (code/static gate)

## Delivered

### Site Search Widget
- domain-bound publishable widget keys for verified Webmaster sites;
- key ensure/rotation/revocation lifecycle;
- exact Origin enforcement and CORS only for the verified origin;
- existing `web_documents` Manticore index reused with exact host filtering;
- defensive host filtering in backend response and browser result links;
- bounded request body/query/result count;
- per-key and per-key+client rate limits;
- embeddable `/poisk-widget.js` using DOM/textContent rather than result HTML injection;
- daily widget usage aggregates without storing query text;
- owner-scoped widget management and analytics.

### CMS integrations
- WordPress adapter: META verification, `[poisk_search]`, widget configuration, sitemap submit and URL SUBMIT/REINDEX/DELETE;
- WordPress Webmaster bearer token stored only as Sodium secretbox ciphertext, with key derived from `AUTH_KEY`, never rendered back to the browser;
- 1C-Bitrix component: widget rendering with escaped parameters;
- 1C-Bitrix META verification hook;
- 1C-Bitrix server-side Webmaster submit client with HTTPS-only API base and no browser credential exposure.

### Agency flow
- explicit agency workspaces and member roles OWNER/MANAGER/ANALYST;
- explicit per-site READ/MANAGE delegation from the verified site owner;
- revocable delegation and agency audit trail;
- tenant isolation tests;
- MANAGE supports delegated sitemap/URL submit without impersonating the client Webmaster account;
- READ does not permit writes; ANALYST does not permit delegated writes;
- delegated URLs must stay on the verified host.

### Organization claiming
- claiming requires the requester's verified Webmaster site;
- exact canonical organization website host must match the verified site host;
- existing Web↔GEO `WEBSITE_HOST` provenance for the same domain/place at confidence 100 is mandatory;
- one active claim per organization;
- claim/revoke events persist evidence and audit history;
- integration tests cover missing provenance, foreign site and owner revoke.

### Referral/campaign attribution
- bounded referral codes and campaign keys;
- one-time attribution tokens, SHA-256 hashes stored in PostgreSQL;
- 24-hour attribution token TTL;
- bounded flow enum and landing-path length;
- daily starts/conversions only; no search query, IP or User-Agent storage in attribution data;
- owner-scoped analytics and referral revocation;
- separate stricter public attribution rate/concurrency/deadline guard;
- integration test covers one-time conversion, owner isolation and revoked referral rejection.

### Operational/security work discovered during Sprint 17
- nginx now overwrites `X-Forwarded-For` with the directly observed client IP so Go `RealIP`/rate limiting cannot be bypassed by a user-supplied forwarding chain in the single-proxy deployment;
- historical duplicate numeric migrations `000018` and `000019` were identified. Because existing deployments may have recorded either duplicate, historical files were not renamed. `000024_historical_migration_repair.sql` idempotently guarantees all required Sprint 16 effects on fresh and partially upgraded databases.

## Tests added
- Widget origin validation unit test.
- Host-scoped Search backend unit test.
- Widget key revocation/rotation and owner-isolation integration test.
- Agency delegation isolation integration test.
- Agency READ/MANAGE/role/foreign-host submit integration test.
- Organization claim proof/provenance integration test.
- Referral one-time attribution and owner isolation integration test.

## Scope controls
- No separate Site Search index was created.
- No Redis/Kafka/RabbitMQ/Kubernetes/Elasticsearch/OpenSearch added.
- No paid ranking or organic ranking influence added.
- No Sprint 18 billing/subscription implementation pulled into Sprint 17.
- No GitHub Actions/CI added; `.github/workflows` is absent.

## Verification note

The repository includes unit/integration test code and the implementation has been statically reviewed against the Sprint 17 contracts. This environment cannot honestly execute the complete Go/PostgreSQL/Manticore integration gate because its container lacks working external dependency/DNS access. Sprint 17 is therefore recorded as a **code/static gate**, not as a claimed runtime integration PASS. Production promotion still requires running the documented integration tests against a fully migrated PostgreSQL/PostGIS + Manticore environment.

## Result

Sprint 17 satisfies its code-level Definition of Done: safe site-search distribution, CMS adapters, explicit agency delegation, provenance-backed organization claims, bounded referral attribution and growth analytics are implemented without weakening the canonical Search/Webmaster/GEO model.
