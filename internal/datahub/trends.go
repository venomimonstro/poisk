package datahub

import (
	"context"
	"time"
)

type Trend struct{
	Day time.Time `json:"day"`
	Query string `json:"query"`
	DemandScore int `json:"demand_score"`
	GapScore int `json:"gap_score"`
	IndependentBuckets int `json:"independent_buckets"`
}

func (r *Repository) MaterializeTrends(ctx context.Context,now time.Time,limit int)(int64,error){
	if r==nil||r.db==nil||limit<1||limit>500{return 0,ErrInvalid};if now.IsZero(){now=time.Now().UTC()};now=now.UTC();day:=time.Date(now.Year(),now.Month(),now.Day(),0,0,0,0,time.UTC)
	tag,err:=r.db.Exec(ctx,`INSERT INTO datahub_trends_daily(day,query_hash,representative_query,demand_score,gap_score,independent_buckets)
SELECT $1,g.query_hash,g.representative_query,g.demand_score,g.gap_score,g.independent_buckets
FROM query_gaps g
WHERE g.representative_query IS NOT NULL
  AND g.state IN ('OPEN','RESOLVED')
  AND g.independent_buckets>=3
  AND g.demand_score>=30
  AND g.last_seen_at >= $2-interval '24 hours'
ORDER BY g.demand_score DESC,g.gap_score DESC,g.query_hash
LIMIT $3
ON CONFLICT(day,query_hash) DO UPDATE SET
 representative_query=EXCLUDED.representative_query,
 demand_score=EXCLUDED.demand_score,
 gap_score=EXCLUDED.gap_score,
 independent_buckets=EXCLUDED.independent_buckets`,day,now,limit);if err!=nil{return 0,err}
	_,err=r.db.Exec(ctx,`DELETE FROM datahub_trends_daily WHERE day < $1-interval '90 days'`,day);if err!=nil{return 0,err};return tag.RowsAffected(),nil
}

func (r *Repository) Trends(ctx context.Context,day time.Time,limit int)([]Trend,error){
	if r==nil||r.db==nil||limit<1||limit>100{return nil,ErrInvalid};if day.IsZero(){now:=time.Now().UTC();day=time.Date(now.Year(),now.Month(),now.Day(),0,0,0,0,time.UTC)}
	rows,err:=r.db.Query(ctx,`SELECT day,representative_query,demand_score,gap_score,independent_buckets FROM datahub_trends_daily WHERE day=$1 ORDER BY demand_score DESC,gap_score DESC,query_hash LIMIT $2`,day,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]Trend,0,limit);for rows.Next(){var t Trend;if err:=rows.Scan(&t.Day,&t.Query,&t.DemandScore,&t.GapScore,&t.IndependentBuckets);err!=nil{return nil,err};out=append(out,t)};return out,rows.Err()
}
