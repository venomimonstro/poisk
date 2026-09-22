package demand

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid=errors.New("invalid demand signal")

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Snapshot struct{
	Normalized string
	Total int64
	AverageQuality float64
	AverageFreshness float64
	AverageSpam float64
	Hosts []string
}

type Gap struct{
	ID int64 `json:"gap_id"`
	State string `json:"state"`
	RepresentativeQuery string `json:"representative_query,omitempty"`
	Scores Scores `json:"scores"`
	IndependentBuckets int `json:"independent_buckets"`
}

func bucketStart(now time.Time)time.Time{if now.IsZero(){now=time.Now().UTC()};now=now.UTC().Truncate(time.Minute);m:=(now.Minute()/10)*10;return time.Date(now.Year(),now.Month(),now.Day(),now.Hour(),m,0,0,time.UTC)}
func hashQuery(q string)[32]byte{return sha256.Sum256([]byte(q))}

func (r *Repository) Record(ctx context.Context,s Snapshot,now time.Time)(Gap,error){
	q:=strings.TrimSpace(s.Normalized);if r==nil||r.db==nil||q==""||len([]rune(q))>256{return Gap{},ErrInvalid}
	if s.Total<0{s.Total=0};s.AverageQuality=clampFloat(s.AverageQuality,0,100);s.AverageFreshness=clampFloat(s.AverageFreshness,0,100);s.AverageSpam=clampFloat(s.AverageSpam,0,100)
	hash:=hashQuery(q);bucket:=bucketStart(now);resultCap:=s.Total;if resultCap>100{resultCap=100}
	zero:=0;if s.Total==0{zero=1};lowQ:=0;if s.AverageQuality<55{lowQ=1};lowF:=0;if s.AverageFreshness<50{lowF=1};highSpam:=0;if s.AverageSpam>60{highSpam=1}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Gap{},err};defer func(){_=tx.Rollback(ctx)}()
	_,err=tx.Exec(ctx,`INSERT INTO query_signal_buckets(query_hash,bucket_start,hits,zero_result_hits,low_quality_hits,low_freshness_hits,high_spam_hits,result_count_sum,quality_sum,freshness_sum,spam_sum)
VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT(query_hash,bucket_start) DO UPDATE SET
 hits=LEAST(3,query_signal_buckets.hits+1),
 zero_result_hits=LEAST(3,query_signal_buckets.zero_result_hits+EXCLUDED.zero_result_hits),
 low_quality_hits=LEAST(3,query_signal_buckets.low_quality_hits+EXCLUDED.low_quality_hits),
 low_freshness_hits=LEAST(3,query_signal_buckets.low_freshness_hits+EXCLUDED.low_freshness_hits),
 high_spam_hits=LEAST(3,query_signal_buckets.high_spam_hits+EXCLUDED.high_spam_hits),
 result_count_sum=LEAST(300,query_signal_buckets.result_count_sum+EXCLUDED.result_count_sum),
 quality_sum=LEAST(300,query_signal_buckets.quality_sum+EXCLUDED.quality_sum),
 freshness_sum=LEAST(300,query_signal_buckets.freshness_sum+EXCLUDED.freshness_sum),
 spam_sum=LEAST(300,query_signal_buckets.spam_sum+EXCLUDED.spam_sum),updated_at=now()`,hash[:],bucket,zero,lowQ,lowF,highSpam,resultCap,int(s.AverageQuality),int(s.AverageFreshness),int(s.AverageSpam));if err!=nil{return Gap{},err}

	for _,host:=range normalizeHosts(s.Hosts){
		_,err=tx.Exec(ctx,`INSERT INTO query_gap_domain_observations(query_hash,domain_id,first_seen_at,last_seen_at,seen_buckets)
SELECT $1,d.domain_id,now(),now(),1 FROM domains d
WHERE d.host=$2 AND d.status='ACTIVE' AND d.policy IN ('ALLOW','LIMITED')
ON CONFLICT(query_hash,domain_id) DO UPDATE SET
 last_seen_at=now(),
 seen_buckets=LEAST(144,query_gap_domain_observations.seen_buckets + CASE WHEN query_gap_domain_observations.last_seen_at < $3 THEN 1 ELSE 0 END)`,hash[:],host,bucket);if err!=nil{return Gap{},err}
	}

	var buckets,hits int;var avgResults,avgQuality,avgFresh,avgSpam float64
	err=tx.QueryRow(ctx,`SELECT count(*),COALESCE(sum(hits),0),
 COALESCE(sum(result_count_sum)::float/NULLIF(sum(hits),0),0),
 COALESCE(sum(quality_sum)::float/NULLIF(sum(hits),0),0),
 COALESCE(sum(freshness_sum)::float/NULLIF(sum(hits),0),0),
 COALESCE(sum(spam_sum)::float/NULLIF(sum(hits),0),0)
FROM query_signal_buckets WHERE query_hash=$1 AND bucket_start>=now()-interval '24 hours'`,hash[:]).Scan(&buckets,&hits,&avgResults,&avgQuality,&avgFresh,&avgSpam);if err!=nil{return Gap{},err}
	scores:=Score(Signals{IndependentBuckets:buckets,Hits:hits,AverageResults:avgResults,AverageQuality:avgQuality,AverageFreshness:avgFresh,AverageSpam:avgSpam})
	qualified:=Qualifies(scores,buckets);state:="WATCH";var representative any=nil;if qualified{state="OPEN";representative=q}
	var out Gap;var rep *string
	err=tx.QueryRow(ctx,`INSERT INTO query_gaps(query_hash,representative_query,state,demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets,qualified_at,last_seen_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CASE WHEN $3='OPEN' THEN now() ELSE NULL END,now())
ON CONFLICT(query_hash) DO UPDATE SET
 representative_query=CASE WHEN query_gaps.state='SUPPRESSED' THEN query_gaps.representative_query WHEN EXCLUDED.state='OPEN' THEN COALESCE(query_gaps.representative_query,EXCLUDED.representative_query) ELSE query_gaps.representative_query END,
 state=CASE WHEN query_gaps.state='SUPPRESSED' THEN 'SUPPRESSED' WHEN EXCLUDED.state='OPEN' THEN 'OPEN' WHEN query_gaps.state='OPEN' THEN 'RESOLVED' ELSE 'WATCH' END,
 demand_score=EXCLUDED.demand_score,coverage_score=EXCLUDED.coverage_score,quality_score=EXCLUDED.quality_score,freshness_score=EXCLUDED.freshness_score,spam_score=EXCLUDED.spam_score,gap_score=EXCLUDED.gap_score,independent_buckets=EXCLUDED.independent_buckets,last_seen_at=now(),
 qualified_at=CASE WHEN EXCLUDED.state='OPEN' THEN COALESCE(query_gaps.qualified_at,now()) ELSE query_gaps.qualified_at END,
 resolved_at=CASE WHEN query_gaps.state='OPEN' AND EXCLUDED.state<>'OPEN' THEN now() ELSE query_gaps.resolved_at END
RETURNING gap_id,state,representative_query,demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets`,hash[:],representative,state,scores.Demand,scores.Coverage,scores.Quality,scores.Freshness,scores.Spam,scores.Gap,buckets).Scan(&out.ID,&out.State,&rep,&out.Scores.Demand,&out.Scores.Coverage,&out.Scores.Quality,&out.Scores.Freshness,&out.Scores.Spam,&out.Scores.Gap,&out.IndependentBuckets);if err!=nil{return Gap{},err};if rep!=nil{out.RepresentativeQuery=*rep}
	if err=tx.Commit(ctx);err!=nil{return Gap{},err};return out,nil
}

func normalizeHosts(in []string)[]string{
	seen:=make(map[string]struct{},8);out:=make([]string,0,8)
	for _,v:=range in{v=strings.ToLower(strings.TrimSpace(v));v=strings.TrimSuffix(v,".");if v==""||len(v)>253{continue};if _,ok:=seen[v];ok{continue};seen[v]=struct{}{};out=append(out,v);if len(out)>=8{break}}
	return out
}
