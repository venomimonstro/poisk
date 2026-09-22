package demand

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type GapStatus struct{
	ID int64 `json:"gap_id"`
	State string `json:"state"`
	RepresentativeQuery string `json:"representative_query,omitempty"`
	Scores Scores `json:"scores"`
	IndependentBuckets int `json:"independent_buckets"`
	FeedbackCount int `json:"feedback_count"`
	LastSeenAt time.Time `json:"last_seen_at"`
	LastFeedbackAt *time.Time `json:"last_feedback_at,omitempty"`
}

func (r *Repository) OpenGaps(ctx context.Context,limit int)([]GapStatus,error){
	if r==nil||r.db==nil||limit<1||limit>500{return nil,ErrInvalid}
	rows,err:=r.db.Query(ctx,`SELECT gap_id,state,COALESCE(representative_query,''),demand_score,coverage_score,quality_score,freshness_score,spam_score,gap_score,independent_buckets,feedback_count,last_seen_at,last_feedback_at
FROM query_gaps WHERE state IN ('OPEN','WATCH') ORDER BY state='OPEN' DESC,gap_score DESC,last_seen_at DESC,gap_id LIMIT $1`,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]GapStatus,0,limit);for rows.Next(){var g GapStatus;if err:=rows.Scan(&g.ID,&g.State,&g.RepresentativeQuery,&g.Scores.Demand,&g.Scores.Coverage,&g.Scores.Quality,&g.Scores.Freshness,&g.Scores.Spam,&g.Scores.Gap,&g.IndependentBuckets,&g.FeedbackCount,&g.LastSeenAt,&g.LastFeedbackAt);err!=nil{return nil,err};out=append(out,g)};return out,rows.Err()
}

func (r *Repository) Suppress(ctx context.Context,gapID int64,reason string)error{
	if r==nil||r.db==nil||gapID<=0{return ErrInvalid};if len(reason)<3||len(reason)>240{return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var state string;err=tx.QueryRow(ctx,`SELECT state FROM query_gaps WHERE gap_id=$1 FOR UPDATE`,gapID).Scan(&state);if errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	if state=="SUPPRESSED"{return tx.Commit(ctx)}
	if _,err=tx.Exec(ctx,`UPDATE query_gaps SET state='SUPPRESSED',suppressed_at=now() WHERE gap_id=$1`,gapID);err!=nil{return err}
	if _,err=tx.Exec(ctx,`DELETE FROM query_gap_domain_feedback WHERE gap_id=$1`,gapID);err!=nil{return err}
	key:=fmt.Sprintf("gap:%d:suppress:%d",gapID,time.Now().UTC().UnixNano())
	if _,err=tx.Exec(ctx,`INSERT INTO query_gap_feedback_events(gap_id,action,idempotency_key,details) VALUES($1,'SUPPRESS',$2,jsonb_build_object('reason',$3,'from_state',$4))`,gapID,key,reason,state);err!=nil{return err}
	return tx.Commit(ctx)
}

func (r *Repository) Reopen(ctx context.Context,gapID int64,reason string)error{
	if r==nil||r.db==nil||gapID<=0{return ErrInvalid};if len(reason)<3||len(reason)>240{return ErrInvalid}
	tx,err:=r.db.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var state string;var gapScore int;err=tx.QueryRow(ctx,`SELECT state,gap_score FROM query_gaps WHERE gap_id=$1 FOR UPDATE`,gapID).Scan(&state,&gapScore);if errors.Is(err,pgx.ErrNoRows){return ErrInvalid};if err!=nil{return err}
	if state!="SUPPRESSED"{return ErrInvalid}
	newState:="WATCH";if gapScore>=30{newState="OPEN"}
	if _,err=tx.Exec(ctx,`UPDATE query_gaps SET state=$2,suppressed_at=NULL,last_feedback_at=NULL WHERE gap_id=$1`,gapID,newState);err!=nil{return err}
	key:=fmt.Sprintf("gap:%d:reopen:%d",gapID,time.Now().UTC().UnixNano())
	if _,err=tx.Exec(ctx,`INSERT INTO query_gap_feedback_events(gap_id,action,idempotency_key,details) VALUES($1,'OPEN',$2,jsonb_build_object('reason',$3,'from_state','SUPPRESSED','to_state',$4))`,gapID,key,reason,newState);err!=nil{return err}
	return tx.Commit(ctx)
}
