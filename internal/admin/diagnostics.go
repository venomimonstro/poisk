package admin

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DiagnosticsSnapshot struct {
	Crawler struct {
		Domains int64 `json:"domains"`
		URLs int64 `json:"urls"`
		Ready int64 `json:"ready"`
		Leased int64 `json:"leased"`
		Retry int64 `json:"retry"`
		Dead int64 `json:"dead"`
		Success24h int64 `json:"success_24h"`
		Retry24h int64 `json:"retry_24h"`
		Dead24h int64 `json:"dead_24h"`
		Blocked24h int64 `json:"blocked_24h"`
	} `json:"crawler"`
	Index struct {
		Ready int64 `json:"ready"`
		Leased int64 `json:"leased"`
		Retry int64 `json:"retry"`
		Dead int64 `json:"dead"`
		Processed24h int64 `json:"processed_24h"`
	} `json:"index"`
	Demand struct {
		Watch int64 `json:"watch"`
		Open int64 `json:"open"`
		Resolved int64 `json:"resolved"`
		Suppressed int64 `json:"suppressed"`
		ActiveFeedback int64 `json:"active_feedback"`
	} `json:"demand"`
	Webmaster struct {
		Users int64 `json:"users"`
		Sites int64 `json:"sites"`
		PendingVerifications int64 `json:"pending_verifications"`
		QueuedSitemaps int64 `json:"queued_sitemaps"`
		QueuedURLRequests int64 `json:"queued_url_requests"`
	} `json:"webmaster"`
	Billing struct {
		Accounts int64 `json:"accounts"`
		ActiveSubscriptions int64 `json:"active_subscriptions"`
		PastDueSubscriptions int64 `json:"past_due_subscriptions"`
		OpenInvoices int64 `json:"open_invoices"`
		FailedInvoices int64 `json:"failed_invoices"`
		UnprocessedPaymentEvents int64 `json:"unprocessed_payment_events"`
	} `json:"billing"`
}

type DiagnosticsRepository struct{ DB *pgxpool.Pool }

func (r DiagnosticsRepository) Snapshot(ctx context.Context) (DiagnosticsSnapshot, error) {
	if r.DB == nil { return DiagnosticsSnapshot{}, errors.New("diagnostics database unavailable") }
	var out DiagnosticsSnapshot
	if err := r.DB.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM domains),
 (SELECT count(*) FROM urls),
 count(*) FILTER(WHERE status='READY'),
 count(*) FILTER(WHERE status='LEASED'),
 count(*) FILTER(WHERE status='RETRY'),
 count(*) FILTER(WHERE status='DEAD')
FROM crawl_queue`).Scan(&out.Crawler.Domains,&out.Crawler.URLs,&out.Crawler.Ready,&out.Crawler.Leased,&out.Crawler.Retry,&out.Crawler.Dead); err != nil { return DiagnosticsSnapshot{}, err }
	if err := r.DB.QueryRow(ctx, `
SELECT
 count(*) FILTER(WHERE outcome='SUCCESS'),
 count(*) FILTER(WHERE outcome='RETRY'),
 count(*) FILTER(WHERE outcome='DEAD'),
 count(*) FILTER(WHERE outcome='BLOCKED')
FROM crawl_history WHERE completed_at>=now()-interval '24 hours'`).Scan(&out.Crawler.Success24h,&out.Crawler.Retry24h,&out.Crawler.Dead24h,&out.Crawler.Blocked24h); err != nil { return DiagnosticsSnapshot{}, err }
	if err := r.DB.QueryRow(ctx, `
SELECT
 count(*) FILTER(WHERE status='READY'),
 count(*) FILTER(WHERE status='LEASED'),
 count(*) FILTER(WHERE status='RETRY'),
 count(*) FILTER(WHERE status='DEAD'),
 count(*) FILTER(WHERE status='PROCESSED' AND processed_at>=now()-interval '24 hours')
FROM index_outbox`).Scan(&out.Index.Ready,&out.Index.Leased,&out.Index.Retry,&out.Index.Dead,&out.Index.Processed24h); err != nil { return DiagnosticsSnapshot{}, err }
	if err := r.DB.QueryRow(ctx, `
SELECT
 count(*) FILTER(WHERE state='WATCH'),
 count(*) FILTER(WHERE state='OPEN'),
 count(*) FILTER(WHERE state='RESOLVED'),
 count(*) FILTER(WHERE state='SUPPRESSED'),
 (SELECT count(*) FROM query_gap_domain_feedback WHERE expires_at>now())
FROM query_gaps`).Scan(&out.Demand.Watch,&out.Demand.Open,&out.Demand.Resolved,&out.Demand.Suppressed,&out.Demand.ActiveFeedback); err != nil { return DiagnosticsSnapshot{}, err }
	if err := r.DB.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM webmaster_users),
 (SELECT count(*) FROM webmaster_sites),
 (SELECT count(*) FROM webmaster_verifications WHERE status='PENDING' AND expires_at>now()),
 (SELECT count(*) FROM webmaster_sitemaps WHERE status IN ('SUBMITTED','RETRY')),
 (SELECT count(*) FROM webmaster_url_requests WHERE status IN ('PENDING','QUEUED'))`).Scan(&out.Webmaster.Users,&out.Webmaster.Sites,&out.Webmaster.PendingVerifications,&out.Webmaster.QueuedSitemaps,&out.Webmaster.QueuedURLRequests); err != nil { return DiagnosticsSnapshot{}, err }
	if err := r.DB.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM billing_accounts WHERE status='ACTIVE'),
 (SELECT count(*) FROM billing_subscriptions WHERE status IN ('ACTIVE','GRACE')),
 (SELECT count(*) FROM billing_subscriptions WHERE status='PAST_DUE'),
 (SELECT count(*) FROM billing_invoices WHERE status='OPEN'),
 (SELECT count(*) FROM billing_invoices WHERE status='FAILED'),
 (SELECT count(*) FROM billing_payment_events WHERE processed_at IS NULL)`).Scan(&out.Billing.Accounts,&out.Billing.ActiveSubscriptions,&out.Billing.PastDueSubscriptions,&out.Billing.OpenInvoices,&out.Billing.FailedInvoices,&out.Billing.UnprocessedPaymentEvents); err != nil { return DiagnosticsSnapshot{}, err }
	return out, nil
}
