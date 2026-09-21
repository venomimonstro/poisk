package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type EffectiveQuota struct {
	Limit int64
	Paid bool
	PlanCode string
	PeriodStart time.Time
	PeriodEnd time.Time
}

func calendarMonth(now time.Time)(time.Time,time.Time){
	if now.IsZero(){now=time.Now().UTC()};now=now.UTC();start:=time.Date(now.Year(),now.Month(),1,0,0,0,0,time.UTC);return start,start.AddDate(0,1,0)
}

func (r *Repository) EffectiveQuota(ctx context.Context,accountID int64,product,metric string,freeLimit int64,now time.Time)(EffectiveQuota,error){
	if r==nil||r.db==nil||accountID<=0||freeLimit<0{return EffectiveQuota{},ErrInvalid};ent,err:=r.Entitlement(ctx,accountID,product,now);if err!=nil{return EffectiveQuota{},err};if ent.Active{if limit,ok:=quotaValue(ent.Quotas,metric);ok{return EffectiveQuota{Limit:limit,Paid:true,PlanCode:ent.PlanCode,PeriodStart:ent.PeriodStart,PeriodEnd:ent.PeriodEnd},nil}}
	start,end:=calendarMonth(now);return EffectiveQuota{Limit:freeLimit,PeriodStart:start,PeriodEnd:end},nil
}

func (r *Repository) ConsumeEffective(ctx context.Context,accountID int64,product,metric string,freeLimit,amount int64,now time.Time)(used int64,q EffectiveQuota,err error){
	if amount<=0{return 0,EffectiveQuota{},ErrInvalid};q,err=r.EffectiveQuota(ctx,accountID,product,metric,freeLimit,now);if err!=nil{return 0,q,err};if q.Limit==0{return 0,q,ErrQuotaExceeded}
	tx,err:=r.db.Begin(ctx);if err!=nil{return 0,q,err};defer func(){_=tx.Rollback(ctx)}();var current int64
	err=tx.QueryRow(ctx,`INSERT INTO billing_usage_periods(account_id,metric_key,period_start,period_end,used) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(account_id,metric_key,period_start,period_end) DO UPDATE SET used=billing_usage_periods.used+EXCLUDED.used,updated_at=now()
WHERE billing_usage_periods.used+EXCLUDED.used<=$6 RETURNING used`,accountID,metric,q.PeriodStart,q.PeriodEnd,amount,q.Limit).Scan(&current)
	if errors.Is(err,pgx.ErrNoRows){return 0,q,ErrQuotaExceeded};if err!=nil{return 0,q,err};if err=tx.Commit(ctx);err!=nil{return 0,q,err};return current,q,nil
}

func (r *Repository) AccountIDForUser(ctx context.Context,userID int64)(int64,error){
	if userID<=0{return 0,ErrInvalid};var id int64;err:=r.db.QueryRow(ctx,`SELECT account_id FROM billing_accounts WHERE webmaster_user_id=$1 AND status='ACTIVE'`,userID).Scan(&id);if errors.Is(err,pgx.ErrNoRows){account,e:=r.EnsureUserAccount(ctx,userID);return account.ID,e};return id,err
}
func (r *Repository) AccountIDForAgency(ctx context.Context,agencyID int64)(int64,error){
	if agencyID<=0{return 0,ErrInvalid};var id int64;err:=r.db.QueryRow(ctx,`SELECT account_id FROM billing_accounts WHERE agency_id=$1 AND status='ACTIVE'`,agencyID).Scan(&id);if errors.Is(err,pgx.ErrNoRows){return 0,ErrNotFound};return id,err
}
