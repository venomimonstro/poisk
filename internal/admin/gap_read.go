package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

type QueryGapRow struct {
	ID int64 `json:"gap_id"`
	Query string `json:"representative_query,omitempty"`
	State string `json:"state"`
	Demand int `json:"demand_score"`
	Coverage int `json:"coverage_score"`
	Quality int `json:"quality_score"`
	Freshness int `json:"freshness_score"`
	Spam int `json:"spam_score"`
	Gap int `json:"gap_score"`
	IndependentBuckets int `json:"independent_buckets"`
	FeedbackCount int `json:"feedback_count"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

func (s Service) ListQueryGaps(ctx context.Context,session Session,state string,limit int)([]QueryGapRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	state=strings.ToUpper(strings.TrimSpace(state));if state!=""&&state!="WATCH"&&state!="OPEN"&&state!="RESOLVED"&&state!="SUPPRESSED"{return nil,ErrInvalidCredential}
	if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT gap_id,COALESCE(representative_query,''),state,demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets,feedback_count,last_seen_at
FROM query_gaps WHERE $1='' OR state=$1 ORDER BY CASE WHEN state='OPEN' THEN 0 WHEN state='WATCH' THEN 1 WHEN state='SUPPRESSED' THEN 2 ELSE 3 END,gap_score DESC,last_seen_at DESC,gap_id DESC LIMIT $2`,state,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]QueryGapRow,0,limit);for rows.Next(){var row QueryGapRow;if err:=rows.Scan(&row.ID,&row.Query,&row.State,&row.Demand,&row.Coverage,&row.Quality,&row.Freshness,&row.Spam,&row.Gap,&row.IndependentBuckets,&row.FeedbackCount,&row.LastSeenAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}
