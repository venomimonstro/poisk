package source

import (
	"context"
	"errors"
	"fmt"
)

func (r *Repository) IsCurrent(ctx context.Context, urlID, version int64) (bool, error) {
	if r == nil || r.db == nil { return false, errors.New("source repository is not initialized") }
	if urlID <= 0 || version <= 0 { return false, errors.New("url id and version must be positive") }
	const q = `SELECT EXISTS (SELECT 1 FROM urls WHERE url_id = $1 AND version = $2)`
	var current bool
	if err := r.db.QueryRow(ctx, q, urlID, version).Scan(&current); err != nil {
		return false, fmt.Errorf("check current URL version: %w", err)
	}
	return current, nil
}
