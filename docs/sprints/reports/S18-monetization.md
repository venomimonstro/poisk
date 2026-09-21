# Sprint 18 — Monetization Report

**Status:** PASS — code/static gate  
**Scope:** Webmaster Pro, Agency, Site Search Pro, Business Pro, provider-neutral billing and usage limits.

## Implemented
- provider-neutral billing accounts for Webmaster users, agencies and claimed organizations;
- versioned plan catalog with integer kopeck pricing and JSON quota snapshots;
- Webmaster Pro 1 490 ₽/month baseline plan;
- Agency 4 990 / 9 990 ₽/month plans;
- Site Search Pro 1 490 / 4 990 ₽/month plans;
- Business Pro 990 / 1 990 ₽/month plans;
- pending subscription + invoice creation separated from payment confirmation;
- idempotent provider payment-event ingestion keyed by `(provider, provider_event_id)` with immutable tuple verification;
- append-only billing ledger for invoices, payments and refunds;
- explicit ACTIVE / GRACE / PAST_DUE / CANCELED / EXPIRED lifecycle;
- immutable DB-triggered subscription status audit trail;
- DB-level one-active-subscription-per-product invariant with upgrade repair for historical duplicates;
- atomic period-bounded usage counters that cannot exceed quota under concurrent consumers;
- Site Search server-side monthly request quota and max-result entitlement enforcement;
- Webmaster server-side site, URL-submit and sitemap-submit limits;
- Agency member/site capacity limits and managed-request monthly quota;
- Business Pro bound to the paying Webmaster user, not an individual place account;
- Business Pro checkout requires an existing active verified organization claim at both API and DB levels;
- claim capacity uses the user's Business Pro entitlement and active-claim count;
- expiry removes paid entitlement without deleting canonical Webmaster/Search/GEO data;
- internal `billingctl` payment/reconcile path; public billing API cannot mark an invoice paid;
- no card/payment credentials stored by Poisk;
- organic ranking and crawl priority are unchanged.

## Security / correctness invariants
- money is stored as `BIGINT` kopecks; no floating-point accounting;
- replayed provider events cannot double-credit the ledger;
- a reused provider event id with altered invoice/type/amount/currency/payload is rejected;
- billing ownership is isolated across users/agencies/claimed organizations;
- quota increments are atomic and race-safe;
- plan expiry falls back to bounded free product limits;
- Business Pro cannot be attached to a PLACE billing account to multiply `claimed_places` quota;
- simultaneous checkout for two plans of the same product is rejected by PostgreSQL unique enforcement;
- no paid organic ranking, paid crawl priority or SERP boost exists.

## Tests / verification
Integration coverage added for:
- payment idempotency and account isolation;
- quota concurrency / hard limit enforcement;
- lifecycle grace and expiry without canonical-data deletion;
- concurrent checkout of two plans for the same product.

A local `go test` run was attempted for the affected packages, but the execution environment could not resolve `github.com`, so the repository/dependencies could not be cloned into the local runner. No GitHub Actions/CI was added. Sprint closure therefore uses the same explicit code/static gate model used by prior environment-constrained sprints rather than claiming an unexecuted runtime test pass.

## Migrations
- `000025_billing_core.sql`
- `000026_site_search_billing_cap.sql`
- `000027_billing_product_uniqueness.sql`
- `000028_billing_subscription_events.sql`

## Rollback / recovery
Billing is additive to canonical Search/Webmaster/GEO state. Disabling billing routes/enforcement does not require deleting canonical product data. Payment events and ledger rows are append-only audit records and should not be destructively rolled back. Subscription/entitlement recovery is performed by reconciliation and compensating events, not by deleting historical money records.

## Scope boundary
Sprint 19 Query Gap / Demand Driven Index work was not pulled into Sprint 18. GitHub Actions/CI were not added.
