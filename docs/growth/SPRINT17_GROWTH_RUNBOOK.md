# Sprint 17 Growth Runbook

Sprint 17 adds distribution and ownership flows on top of the existing Search, Webmaster and GEO canonical data. None of these features changes organic ranking or creates a parallel search index.

## Site Search Widget

1. Verify the site in Webmaster first.
2. Create or recover a publishable widget key with authenticated `POST /api/widget/manage` and action `ENSURE`.
3. Load `/poisk-widget.js` from the Poisk frontend and pass only the `psw_...` publishable key.
4. Public search goes to `POST /api/widget/search?key=...`.

The publishable key is not an account credential. It is useful only from the exact verified site Origin. The API validates active key, verified site, active/non-blocked domain, exact Origin, query limits and rate limits. Manticore retrieval is additionally restricted to the site's exact host.

Rotate a key with action `ROTATE`; revoke with `REVOKE`. Re-enabling a revoked widget creates a new key. Widget usage analytics are daily aggregates only: requests, result count and zero-result count. Search query text is not stored in widget analytics.

## WordPress adapter

Path: `integrations/wordpress/poisk-search/`.

Capabilities:
- META ownership verification;
- `[poisk_search]` embeddable widget shortcode;
- sitemap submit;
- URL `SUBMIT`, `REINDEX`, `DELETE`;
- HTTPS-only Poisk API base.

The publishable widget key may appear in HTML by design. Webmaster bearer credentials do not. The WordPress adapter encrypts the bearer token server-side with Sodium secretbox using a key derived from `AUTH_KEY`, stores the encrypted value with autoload disabled, and never renders the plaintext token back into the admin page.

## 1C-Bitrix adapter

Paths:
- `integrations/bitrix/local/components/poisk/search/`
- `integrations/bitrix/local/php_interface/poisk_search.php`
- `integrations/bitrix/local/php_interface/poisk_webmaster.php`

The verification hook injects the META ownership proof from a Bitrix option. The component accepts only the publishable widget key and HTTPS widget script URL. `PoiskWebmasterClient` performs sitemap/URL submit server-side; callers supply the Webmaster token server-side and should keep it in their deployment secret storage rather than page/component parameters.

## Agency delegation

Agency routes are under `/api/agency` and use the existing Webmaster Bearer session. An agency never receives a client's Webmaster token and never impersonates the site's owner.

The site owner explicitly grants an agency either `READ` or `MANAGE` for one verified site and may revoke it at any time. Agency members are `OWNER`, `MANAGER` or `ANALYST`.

- `READ` permits visibility only.
- `MANAGE` permits delegated sitemap/URL submit only for active `OWNER`/`MANAGER` members.
- `ANALYST` cannot perform delegated writes.
- Submitted URLs must remain on the delegated verified host.

Agency membership, site grants/revocations and delegated submissions have an agency audit trail.

## Organization claiming

Routes are under `/api/claims` and use Webmaster authentication. A claim is accepted only when all of the following are true:

1. the site belongs to the requesting Webmaster user;
2. the site is `VERIFIED` and its domain is active/non-blocked;
3. the organization's canonical website host exactly matches the verified site host;
4. Web↔GEO provenance already contains `WEBSITE_HOST` for the same organization/domain with confidence 100.

Claims and revocations produce immutable `organization_claim_events`. A place can have only one active claim. Claiming does not change organic ranking.

## Referral attribution

Referral owner endpoints are under `/api/growth/referrals`; first-party attribution start/complete is under `/api/growth/attribution`.

Referral codes and campaign keys do not affect Search ranking. Attribution tokens are random one-time tokens; only SHA-256 hashes are stored. Tokens expire after 24 hours. Stored analytics are bounded daily `starts` and `conversions` by referral/campaign/flow. The attribution subsystem does not store search queries, IP addresses or User-Agent strings.

Supported bounded flows: `WEBMASTER_REGISTER`, `SITE_VERIFY`, `WIDGET_ENABLE`, `AGENCY_CREATE`, `ORG_CLAIM`.

## Historical migration repair

Older Sprint 16 work accidentally produced duplicate numeric migration prefixes `000018` and `000019`, while the migration runner records numeric versions. Existing migration files are intentionally left untouched because deployments may already have applied either variant. `000024_historical_migration_repair.sql` is idempotent and converges fresh and partially-upgraded databases to the required final Sprint 16 schema.

Do not rename/delete the historical duplicate migration files on a live deployment without a separate migration-ledger transition plan.

## Security checks before production

- Confirm `/api/widget/search` rejects a copied key from a different Origin.
- Revoke a widget key and confirm it no longer resolves.
- Confirm Agency `READ` and `ANALYST` cannot submit work.
- Confirm Agency `MANAGE` cannot submit a URL on another host.
- Confirm organization claim fails without exact `WEBSITE_HOST` provenance.
- Confirm an attribution token cannot convert twice.
- Confirm nginx overwrites `X-Forwarded-For` with the directly observed client address before Go `RealIP` middleware.
- Run integration tests with `TEST_DATABASE_URL` against the fully migrated PostgreSQL/PostGIS test database.

No GitHub Actions/CI are required or added by Sprint 17.
