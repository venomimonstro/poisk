package reviews

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrRateLimited=errors.New("review action rate limited")

func actionLimit(action string)int{switch action{case "WRITE":return 10;case "REPORT":return 20;case "REPLY":return 10};return 0}

func (r Repository) ConsumeAction(ctx context.Context,userID int64,action string,now time.Time)error{
	if r.DB==nil||userID<=0{return ErrInvalid};action=strings.ToUpper(strings.TrimSpace(action));limit:=actionLimit(action);if limit==0{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};bucket:=now.UTC().Truncate(time.Hour)
	var hits int
	err:=r.DB.QueryRow(ctx,`INSERT INTO organization_review_action_buckets(consumer_user_id,action,bucket_start,hits) VALUES($1,$2,$3,1)
ON CONFLICT(consumer_user_id,action,bucket_start) DO UPDATE SET hits=organization_review_action_buckets.hits+1,updated_at=now()
WHERE organization_review_action_buckets.hits < $4
RETURNING hits`,userID,action,bucket,limit).Scan(&hits)
	if errors.Is(err,pgx.ErrNoRows){return ErrRateLimited};return err
}

func (r Repository) PruneActionBuckets(ctx context.Context,now time.Time)error{if r.DB==nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};_,err:=r.DB.Exec(ctx,`DELETE FROM organization_review_action_buckets WHERE bucket_start<$1`,now.UTC().Add(-7*24*time.Hour).Truncate(time.Hour));return err}
