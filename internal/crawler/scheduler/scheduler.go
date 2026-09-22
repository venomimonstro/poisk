package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DomainBudget struct {
	DomainID          int64
	Host              string
	Policy            string
	CrawlBudget       int32
	MaxURLs           int32
	MaxDepth          int16
	RequestsPerSecond float64
	MaxConcurrency    int16
	MaxBytesPerDay    int64
	QualityScore      float64
	DemandScore       float64
	NextCrawlAt       *time.Time
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EligibleDomains(ctx context.Context, limit int32) ([]DomainBudget, error) {
	if limit <= 0 || limit > 10_000 {
		return nil, errors.New("invalid scheduler domain limit")
	}
	const q = `
WITH active_feedback AS (
    SELECT domain_id, LEAST(25, COALESCE(MAX(boost),0))::double precision AS demand_boost
    FROM query_gap_domain_feedback
    WHERE expires_at > now()
    GROUP BY domain_id
)
SELECT d.domain_id, d.host, d.policy, d.crawl_budget, d.max_urls, d.max_depth,
       d.requests_per_second, d.max_concurrency, d.max_bytes_per_day,
       d.quality_score, d.demand_score + COALESCE(f.demand_boost,0), d.next_crawl_at
FROM domains d
LEFT JOIN active_feedback f ON f.domain_id=d.domain_id
WHERE d.status = 'ACTIVE'
  AND d.policy IN ('ALLOW','LIMITED')
  AND d.crawl_budget > 0
  AND d.max_urls > 0
  AND (d.next_crawl_at IS NULL OR d.next_crawl_at <= now())
ORDER BY (d.demand_score + COALESCE(f.demand_boost,0)) DESC,
         d.quality_score DESC,
         COALESCE(d.next_crawl_at, '-infinity'::timestamptz) ASC,
         d.domain_id ASC
LIMIT $1`

	rows, err := r.db.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("query eligible domains: %w", err)
	}
	defer rows.Close()

	out := make([]DomainBudget, 0, limit)
	for rows.Next() {
		var d DomainBudget
		if err := rows.Scan(
			&d.DomainID,
			&d.Host,
			&d.Policy,
			&d.CrawlBudget,
			&d.MaxURLs,
			&d.MaxDepth,
			&d.RequestsPerSecond,
			&d.MaxConcurrency,
			&d.MaxBytesPerDay,
			&d.QualityScore,
			&d.DemandScore,
			&d.NextCrawlAt,
		); err != nil {
			return nil, fmt.Errorf("scan eligible domain: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible domains: %w", err)
	}
	return out, nil
}

func ComputePriority(demand, expectedQuality, freshnessNeed, changeProbability, fetchCost float64) (float64, error) {
	values := []float64{demand, expectedQuality, freshnessNeed, changeProbability, fetchCost}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, errors.New("priority inputs must be finite")
		}
	}
	if demand < 0 || expectedQuality < 0 || freshnessNeed < 0 || changeProbability < 0 || fetchCost <= 0 {
		return 0, errors.New("invalid negative/zero priority input")
	}

	quality := math.Max(expectedQuality, 0.05)
	freshness := math.Max(freshnessNeed, 0.05)
	change := math.Max(changeProbability, 0.05)
	priority := demand * quality * freshness * change / math.Max(fetchCost, 0.01)
	if priority > 1e12 {
		priority = 1e12
	}
	return priority, nil
}
