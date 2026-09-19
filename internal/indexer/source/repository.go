package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound    = errors.New("document version not found")
	ErrNotIndexable = errors.New("document version is not indexable")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

type Extracted struct {
	URLID          int64
	Version        int64
	Title          string
	Description    string
	Lang           string
	CanonicalURL   string
	CleanText      string
	RobotsNoIndex  bool
	RobotsNoFollow bool
	StructuredData []string
	ContentHash    [32]byte
	SimHash        uint64
	FetchedAt      time.Time
}

type Document struct {
	ID            int64
	Version       int64
	Title         string
	Description   string
	Body          string
	URL           string
	Host          string
	Lang          string
	ContentHash   string
	QualityScore  float64
	SpamScore     float64
	FetchedAt     time.Time
	NoIndex       bool
}

func (r *Repository) SaveExtracted(ctx context.Context, in Extracted) error {
	if r == nil || r.db == nil { return errors.New("source repository is not initialized") }
	if in.URLID <= 0 || in.Version <= 0 { return errors.New("url id and version must be positive") }
	if in.FetchedAt.IsZero() { in.FetchedAt = time.Now().UTC() }
	structured, err := json.Marshal(in.StructuredData)
	if err != nil { return fmt.Errorf("marshal structured data: %w", err) }

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return fmt.Errorf("begin extracted document transaction: %w", err) }
	defer func() { _ = tx.Rollback(ctx) }()

	const upsertContent = `
INSERT INTO document_content (
    url_id, version, title, description, lang, canonical_url, clean_text,
    robots_noindex, robots_nofollow, structured_data, content_hash, simhash, fetched_at
) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10::jsonb,$11,$12,$13)
ON CONFLICT (url_id, version) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    lang = EXCLUDED.lang,
    canonical_url = EXCLUDED.canonical_url,
    clean_text = EXCLUDED.clean_text,
    robots_noindex = EXCLUDED.robots_noindex,
    robots_nofollow = EXCLUDED.robots_nofollow,
    structured_data = EXCLUDED.structured_data,
    content_hash = EXCLUDED.content_hash,
    simhash = EXCLUDED.simhash,
    fetched_at = EXCLUDED.fetched_at`
	if _, err := tx.Exec(ctx, upsertContent,
		in.URLID, in.Version, in.Title, in.Description, in.Lang, in.CanonicalURL, in.CleanText,
		in.RobotsNoIndex, in.RobotsNoFollow, string(structured), in.ContentHash[:], int64(in.SimHash), in.FetchedAt,
	); err != nil {
		return fmt.Errorf("persist extracted document: %w", err)
	}

	const markVersion = `
UPDATE document_versions
SET extraction_status = 'READY',
    content_hash = $3,
    simhash = $4,
    metadata = jsonb_build_object(
        'title', $5::text,
        'description', $6::text,
        'lang', $7::text,
        'canonical_url', NULLIF($8::text,''),
        'robots_noindex', $9::boolean,
        'robots_nofollow', $10::boolean
    )
WHERE url_id = $1 AND version = $2`
	tag, err := tx.Exec(ctx, markVersion,
		in.URLID, in.Version, in.ContentHash[:], int64(in.SimHash), in.Title, in.Description, in.Lang,
		in.CanonicalURL, in.RobotsNoIndex, in.RobotsNoFollow,
	)
	if err != nil { return fmt.Errorf("mark document version ready: %w", err) }
	if tag.RowsAffected() != 1 { return ErrNotFound }

	const updateURL = `
UPDATE urls
SET content_hash = $3,
    simhash = $4,
    index_status = CASE WHEN $5::boolean THEN 'EXCLUDED' ELSE 'NOT_INDEXED' END,
    updated_at = now()
WHERE url_id = $1 AND version = $2`
	if _, err := tx.Exec(ctx, updateURL, in.URLID, in.Version, in.ContentHash[:], int64(in.SimHash), in.RobotsNoIndex); err != nil {
		return fmt.Errorf("update current URL extraction state: %w", err)
	}

	const enqueue = `
INSERT INTO index_outbox (entity_type, entity_id, entity_version, operation, available_at)
VALUES ('WEB_DOCUMENT', $1, $2, 'UPSERT', now())
ON CONFLICT (entity_type, entity_id, entity_version) DO NOTHING`
	if _, err := tx.Exec(ctx, enqueue, in.URLID, in.Version); err != nil {
		return fmt.Errorf("enqueue web document index event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil { return fmt.Errorf("commit extracted document: %w", err) }
	return nil
}

func (r *Repository) LoadVersion(ctx context.Context, urlID, version int64) (Document, error) {
	if r == nil || r.db == nil { return Document{}, errors.New("source repository is not initialized") }
	const q = `
SELECT
    u.url_id,
    dc.version,
    dc.title,
    dc.description,
    dc.clean_text,
    u.normalized_url,
    d.host,
    dc.lang,
    encode(dc.content_hash, 'hex'),
    u.quality_score,
    u.spam_score,
    dc.fetched_at,
    dc.robots_noindex
FROM document_content dc
JOIN document_versions dv ON dv.url_id = dc.url_id AND dv.version = dc.version
JOIN urls u ON u.url_id = dc.url_id
JOIN domains d ON d.domain_id = u.domain_id
WHERE dc.url_id = $1
  AND dc.version = $2
  AND dv.extraction_status = 'READY'`
	var out Document
	err := r.db.QueryRow(ctx, q, urlID, version).Scan(
		&out.ID, &out.Version, &out.Title, &out.Description, &out.Body, &out.URL, &out.Host,
		&out.Lang, &out.ContentHash, &out.QualityScore, &out.SpamScore, &out.FetchedAt, &out.NoIndex,
	)
	if errors.Is(err, pgx.ErrNoRows) { return Document{}, ErrNotFound }
	if err != nil { return Document{}, fmt.Errorf("load document version: %w", err) }
	if out.NoIndex || out.Body == "" { return out, ErrNotIndexable }
	return out, nil
}

func (r *Repository) RebuildBatch(ctx context.Context, afterURLID int64, limit int32) ([]Document, error) {
	if r == nil || r.db == nil { return nil, errors.New("source repository is not initialized") }
	if limit <= 0 || limit > 5000 { return nil, errors.New("rebuild limit must be between 1 and 5000") }
	const q = `
SELECT DISTINCT ON (u.url_id)
    u.url_id,
    dc.version,
    dc.title,
    dc.description,
    dc.clean_text,
    u.normalized_url,
    d.host,
    dc.lang,
    encode(dc.content_hash, 'hex'),
    u.quality_score,
    u.spam_score,
    dc.fetched_at,
    dc.robots_noindex
FROM urls u
JOIN domains d ON d.domain_id = u.domain_id
JOIN document_content dc ON dc.url_id = u.url_id
JOIN document_versions dv ON dv.url_id = dc.url_id AND dv.version = dc.version
WHERE u.url_id > $1
  AND u.index_status <> 'DELETED'
  AND dc.robots_noindex = FALSE
  AND dc.clean_text <> ''
  AND dv.extraction_status = 'READY'
ORDER BY u.url_id ASC, dc.version DESC
LIMIT $2`
	rows, err := r.db.Query(ctx, q, afterURLID, limit)
	if err != nil { return nil, fmt.Errorf("load rebuild batch: %w", err) }
	defer rows.Close()
	out := make([]Document, 0, limit)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.Version, &doc.Title, &doc.Description, &doc.Body, &doc.URL, &doc.Host,
			&doc.Lang, &doc.ContentHash, &doc.QualityScore, &doc.SpamScore, &doc.FetchedAt, &doc.NoIndex,
		); err != nil { return nil, fmt.Errorf("scan rebuild document: %w", err) }
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate rebuild documents: %w", err) }
	return out, nil
}
