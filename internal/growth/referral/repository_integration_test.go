//go:build integration

package referral

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func referralDB(t *testing.T)*pgxpool.Pool{
	t.Helper();dsn:=os.Getenv("TEST_DATABASE_URL");if dsn==""{t.Fatal("TEST_DATABASE_URL is required")};p,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)};if err=p.Ping(context.Background());err!=nil{p.Close();t.Fatal(err)};t.Cleanup(p.Close)
	_,err=p.Exec(context.Background(),`TRUNCATE TABLE growth_attribution_daily,growth_attribution_sessions,growth_referrals,webmaster_users RESTART IDENTITY CASCADE`);if err!=nil{t.Fatal(err)};return p
}
func TestReferralAttributionIsOneTimeAndOwnerScoped(t *testing.T){
	p:=referralDB(t);repo:=NewRepository(p);ctx:=context.Background();var owner,other int64
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash) VALUES('owner@example.test','x') RETURNING user_id`).Scan(&owner);err!=nil{t.Fatal(err)}
	if err:=p.QueryRow(ctx,`INSERT INTO webmaster_users(email,password_hash) VALUES('other@example.test','x') RETURNING user_id`).Scan(&other);err!=nil{t.Fatal(err)}
	ref,err:=repo.Create(ctx,owner,"launch",24*time.Hour);if err!=nil{t.Fatal(err)}
	attr,err:=repo.Start(ctx,ref.Code,"","WIDGET_ENABLE","/webmaster/widget");if err!=nil{t.Fatal(err)}
	if ok,err:=repo.Complete(ctx,attr.Token);err!=nil||!ok{t.Fatalf("complete ok=%v err=%v",ok,err)}
	if _,err:=repo.Complete(ctx,attr.Token);!errors.Is(err,ErrNotFound){t.Fatalf("expected one-time token, err=%v",err)}
	days,err:=repo.DailyForUser(ctx,owner,30);if err!=nil{t.Fatal(err)};if len(days)!=1||days[0].Starts!=1||days[0].Conversions!=1||days[0].Campaign!="launch"{t.Fatalf("owner days=%+v",days)}
	otherDays,err:=repo.DailyForUser(ctx,other,30);if err!=nil{t.Fatal(err)};if len(otherDays)!=0{t.Fatalf("other user saw %+v",otherDays)}
	if err:=repo.Revoke(ctx,other,ref.ID);!errors.Is(err,ErrNotFound){t.Fatalf("foreign revoke err=%v",err)}
	if err:=repo.Revoke(ctx,owner,ref.ID);err!=nil{t.Fatal(err)}
	if _,err:=repo.Start(ctx,ref.Code,"","WIDGET_ENABLE","/");!errors.Is(err,ErrNotFound){t.Fatalf("revoked referral start err=%v",err)}
}
