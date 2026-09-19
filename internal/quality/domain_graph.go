package quality

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DomainGraph struct {
	db *pgxpool.Pool
}

func NewDomainGraph(db *pgxpool.Pool) *DomainGraph { return &DomainGraph{db: db} }

func AuthorityScore(inboundDomains int, inboundWeight float64) float64 {
	if inboundDomains <= 0 || inboundWeight <= 0 || math.IsNaN(inboundWeight) {
		return 0
	}
	score := math.Log1p(inboundWeight)*12 + math.Log1p(float64(inboundDomains))*8
	return roundScore(clamp100(score))
}

func (g *DomainGraph) UpsertEdge(ctx context.Context, sourceDomainID, targetDomainID int64, weight float64) error {
	if g == nil || g.db == nil { return errors.New("domain graph is not initialized") }
	if sourceDomainID <= 0 || targetDomainID <= 0 || sourceDomainID == targetDomainID {
		return errors.New("invalid domain edge")
	}
	if weight <= 0 || weight > 1000000 || math.IsNaN(weight) || math.IsInf(weight, 0) {
		return errors.New("invalid domain edge weight")
	}
	const q = `
INSERT INTO domain_edges (source_domain_id, target_domain_id, weight)
VALUES ($1,$2,$3)
ON CONFLICT (source_domain_id, target_domain_id) DO UPDATE
SET weight = EXCLUDED.weight,
    last_seen_at = now()`
	if _, err := g.db.Exec(ctx, q, sourceDomainID, targetDomainID, weight); err != nil {
		return fmt.Errorf("upsert domain edge: %w", err)
	}
	return nil
}

func (g *DomainGraph) RecomputeAuthority(ctx context.Context) (int64, error) {
	if g == nil || g.db == nil { return 0, errors.New("domain graph is not initialized") }
	const q = `
WITH inbound AS (
    SELECT target_domain_id,
           COUNT(DISTINCT source_domain_id)::bigint AS inbound_domains,
           SUM(weight)::double precision AS inbound_weight
    FROM domain_edges
    GROUP BY target_domain_id
), scores AS (
    SELECT d.domain_id,
           LEAST(100::double precision,
                 GREATEST(0::double precision,
                    LN(1 + COALESCE(i.inbound_weight, 0)) * 12
                    + LN(1 + COALESCE(i.inbound_domains, 0)) * 8
                 )) AS authority_score
    FROM domains d
    LEFT JOIN inbound i ON i.target_domain_id = d.domain_id
)
UPDATE domains d
SET authority_score = s.authority_score,
    updated_at = now()
FROM scores s
WHERE d.domain_id = s.domain_id
  AND d.authority_score IS DISTINCT FROM s.authority_score`
	tag, err := g.db.Exec(ctx, q)
	if err != nil { return 0, fmt.Errorf("recompute domain authority: %w", err) }
	return tag.RowsAffected(), nil
}
