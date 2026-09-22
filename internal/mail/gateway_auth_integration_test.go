//go:build integration

package mail

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGatewayReplayGuardClassifiesReplayAndMismatch(t *testing.T){
	pool:=mailDB(t);repo:=Repository{DB:pool};ctx:=context.Background();now:=time.Now().UTC();eventID:="evt-integration-0123456789abcdef";first:=GatewayBodySHA256([]byte("first"));second:=GatewayBodySHA256([]byte("second"))
	if err:=repo.ClaimGatewayEvent(ctx,eventID,first,now);err!=nil{t.Fatal(err)}
	if err:=repo.ClaimGatewayEvent(ctx,eventID,first,now.Add(time.Second));!errors.Is(err,ErrGatewayReplay){t.Fatalf("same replay err=%v",err)}
	if err:=repo.ClaimGatewayEvent(ctx,eventID,second,now.Add(2*time.Second));!errors.Is(err,ErrGatewayMismatch){t.Fatalf("mismatch err=%v",err)}
	if _,err:=pool.Exec(ctx,`UPDATE mail_gateway_replay_guard SET expires_at=$2 WHERE event_id=$1`,eventID,now.Add(-time.Second));err!=nil{t.Fatal(err)}
	if err:=repo.PruneGatewayReplayGuard(ctx,now);err!=nil{t.Fatal(err)}
	var count int;if err:=pool.QueryRow(ctx,`SELECT count(*) FROM mail_gateway_replay_guard WHERE event_id=$1`,eventID).Scan(&count);err!=nil{t.Fatal(err)};if count!=0{t.Fatalf("replay row retained=%d",count)}
}
