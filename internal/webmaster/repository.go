package webmaster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("webmaster entity not found")
	ErrConflict     = errors.New("webmaster entity already exists")
	ErrNotVerified  = errors.New("site ownership is not verified")
	ErrUnauthorized = errors.New("webmaster authentication required")
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Status       string
}

type Site struct {
	ID                 int64      `json:"site_id"`
	UserID             int64      `json:"-"`
	DomainID           int64      `json:"domain_id"`
	Origin             string     `json:"origin"`
	Host               string     `json:"host"`
	Status             string     `json:"status"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
	VerificationMethod string     `json:"verification_method,omitempty"`
}

type Verification struct {
	ID        int64     `json:"verification_id"`
	SiteID    int64     `json:"site_id"`
	Method    string    `json:"method"`
	TokenHint string    `json:"token_hint"`
	ExpiresAt time.Time `json:"expires_at"`
}

type URLStatus struct {
	URLID          *int64     `json:"url_id,omitempty"`
	URL            string     `json:"url"`
	CrawlStatus    string     `json:"crawl_status"`
	IndexStatus    string     `json:"index_status"`
	HTTPStatus     *int16     `json:"http_status,omitempty"`
	LastCrawlAt    *time.Time `json:"last_crawl_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	CanonicalURL   string     `json:"canonical_url,omitempty"`
	RobotsNoIndex  bool       `json:"robots_noindex"`
	RobotsNoFollow bool       `json:"robots_nofollow"`
}

type Metrics struct {
	Impressions     int64   `json:"impressions"`
	Clicks          int64   `json:"clicks"`
	AnswerCitations int64   `json:"answer_citations"`
	CTR             float64 `json:"ctr"`
}

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	if r == nil || r.db == nil { return User{}, errors.New("webmaster repository is not initialized") }
	email = strings.ToLower(strings.TrimSpace(email))
	var out User
	err := r.db.QueryRow(ctx, `INSERT INTO webmaster_users(email,password_hash) VALUES($1,$2) RETURNING user_id,email,password_hash,status`, email, passwordHash).
		Scan(&out.ID, &out.Email, &out.PasswordHash, &out.Status)
	if isUniqueViolation(err) { return User{}, ErrConflict }
	if err != nil { return User{}, fmt.Errorf("create webmaster user: %w", err) }
	return out, nil
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (User, error) {
	if r == nil || r.db == nil { return User{}, errors.New("webmaster repository is not initialized") }
	var out User
	err := r.db.QueryRow(ctx, `SELECT user_id,email,password_hash,status FROM webmaster_users WHERE lower(email)=lower($1)`, strings.TrimSpace(email)).
		Scan(&out.ID, &out.Email, &out.PasswordHash, &out.Status)
	if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrNotFound }
	if err != nil { return User{}, fmt.Errorf("load webmaster user: %w", err) }
	return out, nil
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, tokenHash [32]byte, expires time.Time) error {
	if userID <= 0 || expires.IsZero() { return errors.New("invalid session input") }
	_, err := r.db.Exec(ctx, `INSERT INTO webmaster_sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)`, userID, tokenHash[:], expires)
	if err != nil { return fmt.Errorf("create webmaster session: %w", err) }
	return nil
}

func (r *Repository) UserBySession(ctx context.Context, tokenHash [32]byte) (User, error) {
	var out User
	err := r.db.QueryRow(ctx, `
SELECT u.user_id,u.email,u.password_hash,u.status
FROM webmaster_sessions s
JOIN webmaster_users u ON u.user_id=s.user_id
WHERE s.token_hash=$1 AND s.expires_at>now() AND u.status='ACTIVE'`, tokenHash[:]).Scan(&out.ID,&out.Email,&out.PasswordHash,&out.Status)
	if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrUnauthorized }
	if err != nil { return User{}, fmt.Errorf("resolve webmaster session: %w", err) }
	_, _ = r.db.Exec(ctx, `UPDATE webmaster_sessions SET last_seen_at=now() WHERE token_hash=$1`, tokenHash[:])
	return out, nil
}

func (r *Repository) DeleteSession(ctx context.Context, userID int64, tokenHash [32]byte) error {
	_, err := r.db.Exec(ctx, `DELETE FROM webmaster_sessions WHERE user_id=$1 AND token_hash=$2`, userID, tokenHash[:])
	if err != nil { return fmt.Errorf("delete webmaster session: %w", err) }
	return nil
}

func (r *Repository) AddSite(ctx context.Context, userID int64, origin SiteOrigin) (Site, error) {
	if userID <= 0 || origin.Host == "" || origin.Origin == "" { return Site{}, errors.New("invalid site input") }
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Site{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var domainID int64
	if err := tx.QueryRow(ctx, `INSERT INTO domains(host) VALUES($1) ON CONFLICT(host) DO UPDATE SET host=EXCLUDED.host RETURNING domain_id`, origin.Host).Scan(&domainID); err != nil {
		return Site{}, fmt.Errorf("ensure site domain: %w", err)
	}
	var out Site
	err = tx.QueryRow(ctx, `
INSERT INTO webmaster_sites(user_id,domain_id,origin,host)
VALUES($1,$2,$3,$4)
RETURNING site_id,user_id,domain_id,origin,host,status,verified_at,COALESCE(verification_method,'')`, userID, domainID, origin.Origin, origin.Host).
		Scan(&out.ID,&out.UserID,&out.DomainID,&out.Origin,&out.Host,&out.Status,&out.VerifiedAt,&out.VerificationMethod)
	if isUniqueViolation(err) { return Site{}, ErrConflict }
	if err != nil { return Site{}, fmt.Errorf("create webmaster site: %w", err) }
	if err := auditTx(ctx, tx, userID, "WEBMASTER_SITE_ADD", "WEBMASTER_SITE", out.ID, map[string]string{"host": out.Host}); err != nil { return Site{}, err }
	if err := tx.Commit(ctx); err != nil { return Site{}, err }
	return out, nil
}

func (r *Repository) ListSites(ctx context.Context, userID int64) ([]Site, error) {
	rows, err := r.db.Query(ctx, `SELECT site_id,user_id,domain_id,origin,host,status,verified_at,COALESCE(verification_method,'') FROM webmaster_sites WHERE user_id=$1 ORDER BY site_id`, userID)
	if err != nil { return nil, fmt.Errorf("list webmaster sites: %w", err) }
	defer rows.Close()
	var out []Site
	for rows.Next() {
		var s Site
		if err := rows.Scan(&s.ID,&s.UserID,&s.DomainID,&s.Origin,&s.Host,&s.Status,&s.VerifiedAt,&s.VerificationMethod); err != nil { return nil, err }
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) OwnedSite(ctx context.Context, userID, siteID int64, requireVerified bool) (Site, error) {
	q := `SELECT site_id,user_id,domain_id,origin,host,status,verified_at,COALESCE(verification_method,'') FROM webmaster_sites WHERE site_id=$1 AND user_id=$2`
	if requireVerified { q += ` AND status='VERIFIED'` }
	var s Site
	err := r.db.QueryRow(ctx, q, siteID, userID).Scan(&s.ID,&s.UserID,&s.DomainID,&s.Origin,&s.Host,&s.Status,&s.VerifiedAt,&s.VerificationMethod)
	if errors.Is(err, pgx.ErrNoRows) {
		if requireVerified { return Site{}, ErrNotVerified }
		return Site{}, ErrNotFound
	}
	if err != nil { return Site{}, fmt.Errorf("load owned site: %w", err) }
	return s, nil
}

func (r *Repository) CreateVerification(ctx context.Context, userID, siteID int64, method string, tokenHash [32]byte, hint string, expires time.Time) (Verification, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return Verification{}, err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var owned int64
	if err := tx.QueryRow(ctx, `SELECT site_id FROM webmaster_sites WHERE site_id=$1 AND user_id=$2 AND status<>'SUSPENDED' FOR UPDATE`, siteID,userID).Scan(&owned); errors.Is(err, pgx.ErrNoRows) { return Verification{}, ErrNotFound } else if err != nil { return Verification{}, err }
	if _, err := tx.Exec(ctx, `UPDATE webmaster_verifications SET status='REVOKED' WHERE site_id=$1 AND method=$2 AND status='PENDING'`, siteID, method); err != nil { return Verification{}, err }
	var out Verification
	err = tx.QueryRow(ctx, `INSERT INTO webmaster_verifications(site_id,token_hash,token_hint,method,expires_at) VALUES($1,$2,$3,$4,$5) RETURNING verification_id,site_id,method,token_hint,expires_at`, siteID,tokenHash[:],hint,method,expires).
		Scan(&out.ID,&out.SiteID,&out.Method,&out.TokenHint,&out.ExpiresAt)
	if err != nil { return Verification{}, fmt.Errorf("create ownership verification: %w", err) }
	if err := auditTx(ctx,tx,userID,"WEBMASTER_VERIFICATION_CREATE","WEBMASTER_SITE",siteID,map[string]string{"method":method}); err != nil { return Verification{}, err }
	if err := tx.Commit(ctx); err != nil { return Verification{}, err }
	return out,nil
}

func (r *Repository) MarkVerified(ctx context.Context, userID, siteID int64, method string, tokenHash [32]byte) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return err }
	defer func(){ _ = tx.Rollback(ctx) }()
	var verificationID int64
	err = tx.QueryRow(ctx, `
UPDATE webmaster_verifications v
SET status='VERIFIED', verified_at=now()
FROM webmaster_sites s
WHERE v.site_id=s.site_id AND s.site_id=$1 AND s.user_id=$2
  AND v.method=$3 AND v.token_hash=$4 AND v.status='PENDING' AND v.expires_at>now()
RETURNING v.verification_id`, siteID,userID,method,tokenHash[:]).Scan(&verificationID)
	if errors.Is(err,pgx.ErrNoRows) { return ErrNotFound }
	if err != nil { return err }
	if _, err := tx.Exec(ctx, `UPDATE webmaster_sites SET status='VERIFIED',verified_at=now(),verification_method=$3,updated_at=now() WHERE site_id=$1 AND user_id=$2`, siteID,userID,method); err != nil { return err }
	if err := auditTx(ctx,tx,userID,"WEBMASTER_SITE_VERIFIED","WEBMASTER_SITE",siteID,map[string]string{"method":method}); err != nil { return err }
	return tx.Commit(ctx)
}

func (r *Repository) SubmitSitemap(ctx context.Context, userID, siteID int64, sitemapURL string) (int64,error) {
	var id int64
	err := r.db.QueryRow(ctx, `
INSERT INTO webmaster_sitemaps(site_id,sitemap_url)
SELECT s.site_id,$3 FROM webmaster_sites s WHERE s.site_id=$1 AND s.user_id=$2 AND s.status='VERIFIED'
ON CONFLICT(site_id,sitemap_url) DO UPDATE SET status='SUBMITTED',last_error=NULL,submitted_at=now(),updated_at=now()
RETURNING sitemap_id`, siteID,userID,sitemapURL).Scan(&id)
	if errors.Is(err,pgx.ErrNoRows) { return 0,ErrNotVerified }
	if err != nil { return 0,fmt.Errorf("submit sitemap: %w",err) }
	return id,nil
}

func (r *Repository) SubmitURLRequest(ctx context.Context, userID, siteID int64, normalizedURL, operation string) (int64,error) {
	var id int64
	err := r.db.QueryRow(ctx, `
INSERT INTO webmaster_url_requests(site_id,url_id,normalized_url,operation)
SELECT s.site_id,u.url_id,$3,$4
FROM webmaster_sites s
LEFT JOIN urls u ON u.normalized_url=$3
WHERE s.site_id=$1 AND s.user_id=$2 AND s.status='VERIFIED'
RETURNING request_id`, siteID,userID,normalizedURL,operation).Scan(&id)
	if isUniqueViolation(err) { return 0,ErrConflict }
	if errors.Is(err,pgx.ErrNoRows) { return 0,ErrNotVerified }
	if err != nil { return 0,fmt.Errorf("submit URL request: %w",err) }
	return id,nil
}

func (r *Repository) URLStatus(ctx context.Context, userID, siteID int64, normalizedURL string) (URLStatus,error) {
	var out URLStatus
	err := r.db.QueryRow(ctx, `
SELECT u.url_id,$3,COALESCE(u.crawl_status,'DISCOVERED'),COALESCE(u.index_status,'NOT_INDEXED'),u.http_status,u.last_crawl_at,
       COALESCE((SELECT ch.error_code FROM crawl_history ch WHERE ch.url_id=u.url_id ORDER BY ch.completed_at DESC LIMIT 1),''),
       COALESCE(dc.canonical_url,''),COALESCE(dc.robots_noindex,false),COALESCE(dc.robots_nofollow,false)
FROM webmaster_sites s
LEFT JOIN urls u ON u.domain_id=s.domain_id AND u.normalized_url=$3
LEFT JOIN LATERAL (
    SELECT canonical_url,robots_noindex,robots_nofollow
    FROM document_content
    WHERE url_id=u.url_id
    ORDER BY version DESC
    LIMIT 1
) dc ON true
WHERE s.site_id=$1 AND s.user_id=$2`, siteID,userID,normalizedURL).
		Scan(&out.URLID,&out.URL,&out.CrawlStatus,&out.IndexStatus,&out.HTTPStatus,&out.LastCrawlAt,&out.LastError,&out.CanonicalURL,&out.RobotsNoIndex,&out.RobotsNoFollow)
	if errors.Is(err,pgx.ErrNoRows) { return URLStatus{},ErrNotFound }
	if err != nil { return URLStatus{},fmt.Errorf("load URL status: %w",err) }
	return out,nil
}

func (r *Repository) Metrics(ctx context.Context, userID, siteID int64, from, to time.Time) (Metrics,error) {
	var out Metrics
	err := r.db.QueryRow(ctx, `
SELECT COALESCE(sum(m.impressions),0),COALESCE(sum(m.clicks),0),COALESCE(sum(m.answer_citations),0)
FROM webmaster_sites s
LEFT JOIN webmaster_metrics_daily m ON m.site_id=s.site_id AND m.day BETWEEN $3::date AND $4::date
WHERE s.site_id=$1 AND s.user_id=$2
GROUP BY s.site_id`,siteID,userID,from,to).Scan(&out.Impressions,&out.Clicks,&out.AnswerCitations)
	if errors.Is(err,pgx.ErrNoRows) { return Metrics{},ErrNotFound }
	if err != nil { return Metrics{},err }
	if out.Impressions>0 { out.CTR=float64(out.Clicks)/float64(out.Impressions) }
	return out,nil
}

func auditTx(ctx context.Context, tx pgx.Tx, userID int64, action, entityType string, entityID int64, details any) error {
	_,err:=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details) VALUES('USER',$1,$2,$3,$4,to_jsonb($5::json))`,fmt.Sprint(userID),action,entityType,fmt.Sprint(entityID),mustJSON(details))
	if err!=nil { return fmt.Errorf("write webmaster audit: %w",err) }
	return nil
}

func mustJSON(v any) string {
	// Details are intentionally tiny fixed server-side maps; avoid propagating an
	// encoding dependency through every repository method.
	switch m:=v.(type) {
	case map[string]string:
		parts:=make([]string,0,len(m))
		for k,val:=range m { parts=append(parts,fmt.Sprintf("%q:%q",k,val)) }
		return "{"+strings.Join(parts,",")+"}"
	default:
		return "{}"
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err,&pgErr) && pgErr.Code=="23505"
}
