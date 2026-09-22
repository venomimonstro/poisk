package demand

import (
	"context"
	"fmt"
	"time"
)

type FeedbackStats struct{
	Boosted int64 `json:"boosted"`
	Expired int64 `json:"expired"`
	PrunedBuckets int64 `json:"pruned_buckets"`
	PrunedObservations int64 `json:"pruned_observations"`
}

func (r *Repository) MaterializeFeedback(ctx context.Context,limit int,now time.Time)(FeedbackStats,error){
	if r==nil||r.db==nil||limit<1||limit>500{return FeedbackStats{},ErrInvalid}
	if now.IsZero(){now=time.Now().UTC()};now=now.UTC()
	tx,err:=r.db.Begin(ctx);if err!=nil{return FeedbackStats{},err};defer func(){_=tx.Rollback(ctx)}()

	expiredTag,err:=tx.Exec(ctx,`DELETE FROM query_gap_domain_feedback WHERE expires_at <= $1`,now);if err!=nil{return FeedbackStats{},err}
	bucketTag,err:=tx.Exec(ctx,`DELETE FROM query_signal_buckets WHERE bucket_start < $1-interval '7 days'`,now);if err!=nil{return FeedbackStats{},err}
	observationTag,err:=tx.Exec(ctx,`DELETE FROM query_gap_domain_observations WHERE last_seen_at < $1-interval '7 days'`,now);if err!=nil{return FeedbackStats{},err}

	rows,err:=tx.Query(ctx,`SELECT g.gap_id,o.domain_id,g.gap_score,g.coverage_score,g.quality_score,g.freshness_score
FROM query_gaps g
JOIN query_gap_domain_observations o ON o.query_hash=g.query_hash
JOIN domains d ON d.domain_id=o.domain_id
WHERE g.state='OPEN'
  AND g.independent_buckets>=3
  AND g.gap_score>=45
  AND o.seen_buckets>=2
  AND o.last_seen_at >= $1-interval '24 hours'
  AND d.status='ACTIVE' AND d.policy IN ('ALLOW','LIMITED')
  AND (g.last_feedback_at IS NULL OR g.last_feedback_at <= $1-interval '30 minutes')
ORDER BY g.gap_score DESC,g.last_seen_at DESC,g.gap_id,o.domain_id
LIMIT $2`,now,limit);if err!=nil{return FeedbackStats{},err}
	type candidate struct{gapID,domainID int64;gap,coverage,quality,freshness int}
	items:=make([]candidate,0,limit)
	for rows.Next(){var c candidate;if err:=rows.Scan(&c.gapID,&c.domainID,&c.gap,&c.coverage,&c.quality,&c.freshness);err!=nil{rows.Close();return FeedbackStats{},err};items=append(items,c)}
	if err:=rows.Err();err!=nil{rows.Close();return FeedbackStats{},err};rows.Close()

	var boosted int64
	for _,c:=range items{
		boost:=c.gap/4;if boost<1{boost=1};if boost>25{boost=25}
		reason:="LOW_COVERAGE";lowest:=c.coverage
		if c.quality<lowest{reason="LOW_QUALITY";lowest=c.quality}
		if c.freshness<lowest{reason="LOW_FRESHNESS"}
		expires:=now.Add(6*time.Hour)
		key:=fmt.Sprintf("gap:%d:domain:%d:%s",c.gapID,c.domainID,now.Format("2006010215"))
		tag,err:=tx.Exec(ctx,`INSERT INTO query_gap_feedback_events(gap_id,domain_id,action,boost,idempotency_key,details)
VALUES($1,$2,'BOOST_DOMAIN',$3,$4,jsonb_build_object('reason',$5,'expires_at',$6)) ON CONFLICT(idempotency_key) DO NOTHING`,c.gapID,c.domainID,boost,key,reason,expires);if err!=nil{return FeedbackStats{},err}
		if tag.RowsAffected()==0{continue}
		_,err=tx.Exec(ctx,`INSERT INTO query_gap_domain_feedback(gap_id,domain_id,boost,expires_at,reason,updated_at)
VALUES($1,$2,$3,$4,$5,now())
ON CONFLICT(gap_id,domain_id) DO UPDATE SET boost=GREATEST(query_gap_domain_feedback.boost,EXCLUDED.boost),expires_at=GREATEST(query_gap_domain_feedback.expires_at,EXCLUDED.expires_at),reason=EXCLUDED.reason,updated_at=now()`,c.gapID,c.domainID,boost,expires,reason);if err!=nil{return FeedbackStats{},err}
		boosted++
		_,err=tx.Exec(ctx,`UPDATE domains SET next_crawl_at=LEAST(COALESCE(next_crawl_at,$1+interval '30 minutes'),$1+interval '30 minutes'),updated_at=now() WHERE domain_id=$2 AND status='ACTIVE' AND policy IN ('ALLOW','LIMITED')`,now,c.domainID);if err!=nil{return FeedbackStats{},err}
		_,err=tx.Exec(ctx,`UPDATE query_gaps SET last_feedback_at=$1,feedback_count=LEAST(1000,feedback_count+1) WHERE gap_id=$2`,now,c.gapID);if err!=nil{return FeedbackStats{},err}
	}
	if err=tx.Commit(ctx);err!=nil{return FeedbackStats{},err}
	return FeedbackStats{Boosted:boosted,Expired:expiredTag.RowsAffected(),PrunedBuckets:bucketTag.RowsAffected(),PrunedObservations:observationTag.RowsAffected()},nil
}
