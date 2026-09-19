package source

import (
	"context"
	"errors"
	"fmt"
)

// RebuildCurrentBatch enumerates only the current URL version. It must be used
// by rebuild jobs instead of historical document_content rows.
func (r *Repository) RebuildCurrentBatch(ctx context.Context, afterURLID int64, limit int32) ([]Document, error) {
	if r == nil || r.db == nil { return nil, errors.New("source repository is not initialized") }
	if limit <= 0 || limit > 5000 { return nil, errors.New("rebuild limit must be between 1 and 5000") }
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
FROM urls u
JOIN domains d ON d.domain_id = u.domain_id
JOIN document_content dc ON dc.url_id = u.url_id AND dc.version = u.version
JOIN document_versions dv ON dv.url_id = dc.url_id AND dv.version = dc.version
WHERE u.url_id > $1
  AND u.index_status <> 'DELETED'
  AND dc.robots_noindex = FALSE
  AND dc.clean_text <> ''
  AND dv.extraction_status = 'READY'
ORDER BY u.url_id ASC
LIMIT $2`
	rows, err := r.db.Query(ctx, q, afterURLID, limit)
	if err != nil { return nil, fmt.Errorf("load current rebuild batch: %w", err) }
	defer rows.Close()
	out := make([]Document, 0, limit)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.Version, &doc.Title, &doc.Description, &doc.Body, &doc.URL, &doc.Host,
			&doc.Lang, &doc.ContentHash, &doc.QualityScore, &doc.SpamScore, &doc.FetchedAt, &doc.NoIndex,
		); err != nil { return nil, fmt.Errorf("scan current rebuild document: %w", err) }
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate current rebuild documents: %w", err) }
	return out, nil
}
