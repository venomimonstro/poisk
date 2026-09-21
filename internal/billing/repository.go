package billing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid = errors.New("invalid billing input")
	ErrForbidden = errors.New("billing owner forbidden")
	ErrNotFound = errors.New("billing object not found")
	ErrConflict = errors.New("billing conflict")
	ErrQuotaExceeded = errors.New("billing quota exceeded")
	ErrPaymentMismatch = errors.New("payment event does not match invoice")
)

type Repository struct{db *pgxpool.Pool}
func NewRepository(db *pgxpool.Pool)*Repository{return &Repository{db:db}}

type Account struct{ID int64 `json:"account_id"`;OwnerType string `json:"owner_type"`;OwnerID int64 `json:"owner_id"`;Status string `json:"status"`}
type Plan struct{ID int64 `json:"plan_id"`;Product string `json:"product"`;Code string `json:"code"`;Version int `json:"version"`;PriceKopecks int64 `json:"monthly_price_kopecks"`;Currency string `json:"currency"`;Quotas map[string]int64 `json:"quotas"`}
type Subscription struct{ID int64 `json:"subscription_id"`;AccountID int64 `json:"account_id"`;PlanID int64 `json:"plan_id"`;Status string `json:"status"`;PeriodStart time.Time `json:"period_start"`;PeriodEnd time.Time `json:"period_end"`;GraceUntil *time.Time `json:"grace_until,omitempty"`}
type Invoice struct{ID int64 `json:"invoice_id"`;AccountID int64 `json:"account_id"`;SubscriptionID int64 `json:"subscription_id"`;Status string `json:"status"`;AmountKopecks int64 `json:"amount_kopecks"`;Currency string `json:"currency"`;ExternalReference string `json:"external_reference"`}
type Entitlement struct{Active bool `json:"active"`;Product string `json:"product"`;PlanCode string `json:"plan_code,omitempty"`;PlanVersion int `json:"plan_version,omitempty"`;PeriodStart time.Time `json:"period_start,omitempty"`;PeriodEnd time.Time `json:"period_end,omitempty"`;Quotas map[string]int64 `json:"quotas,omitempty"`}

func (r *Repository) EnsureUserAccount(ctx context.Context,userID int64)(Account,error){return r.ensureAccount(ctx,"USER",userID,userID,0,0)}
func (r *Repository) EnsureAgencyAccount(ctx context.Context,userID,agencyID int64)(Account,error){
	if r==nil||r.db==nil||userID<=0||agencyID<=0{return Account{},ErrInvalid};var ok bool
	if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM agency_members m JOIN agencies a ON a.agency_id=m.agency_id WHERE m.agency_id=$1 AND m.user_id=$2 AND m.status='ACTIVE' AND m.role='OWNER' AND a.status='ACTIVE')`,agencyID,userID).Scan(&ok);err!=nil{return Account{},err};if !ok{return Account{},ErrForbidden};return r.ensureAccount(ctx,"AGENCY",agencyID,0,agencyID,0)
}
func (r *Repository) EnsurePlaceAccount(ctx context.Context,userID,placeID int64)(Account,error){
	if r==nil||r.db==nil||userID<=0||placeID<=0{return Account{},ErrInvalid};var ok bool
	if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims c JOIN organizations o ON o.place_id=c.place_id WHERE c.place_id=$1 AND c.user_id=$2 AND c.status='ACTIVE' AND o.status IN ('ACTIVE','REVIEW'))`,placeID,userID).Scan(&ok);err!=nil{return Account{},err};if !ok{return Account{},ErrForbidden};return r.ensureAccount(ctx,"PLACE",placeID,0,0,placeID)
}
func (r *Repository) ensureAccount(ctx,kind string,ownerID,userID,agencyID,placeID int64)(Account,error){
	if r==nil||r.db==nil||ownerID<=0{return Account{},ErrInvalid};var out Account
	var err error
	switch kind{
	case "USER":err=r.db.QueryRow(ctx,`INSERT INTO billing_accounts(webmaster_user_id) VALUES($1) ON CONFLICT(webmaster_user_id) WHERE webmaster_user_id IS NOT NULL DO UPDATE SET updated_at=now() RETURNING account_id,status`,userID).Scan(&out.ID,&out.Status)
	case "AGENCY":err=r.db.QueryRow(ctx,`INSERT INTO billing_accounts(agency_id) VALUES($1) ON CONFLICT(agency_id) WHERE agency_id IS NOT NULL DO UPDATE SET updated_at=now() RETURNING account_id,status`,agencyID).Scan(&out.ID,&out.Status)
	case "PLACE":err=r.db.QueryRow(ctx,`INSERT INTO billing_accounts(place_id) VALUES($1) ON CONFLICT(place_id) WHERE place_id IS NOT NULL DO UPDATE SET updated_at=now() RETURNING account_id,status`,placeID).Scan(&out.ID,&out.Status)
	default:return Account{},ErrInvalid
	};if err!=nil{return Account{},err};out.OwnerType=kind;out.OwnerID=ownerID;return out,nil
}

func (r *Repository) Plans(ctx context.Context,product string)([]Plan,error){
	product=strings.ToUpper(strings.TrimSpace(product));args:=[]any{};where:="status='ACTIVE'";if product!=""{where+=" AND product_code=$1";args=append(args,product)}
	rows,err:=r.db.Query(ctx,`SELECT plan_id,product_code,plan_code,version,monthly_price_kopecks,currency,quotas FROM billing_plans WHERE `+where+` ORDER BY product_code,monthly_price_kopecks,version DESC`,args...);if err!=nil{return nil,err};defer rows.Close();out:=[]Plan{};for rows.Next(){var p Plan;var raw []byte;if err:=rows.Scan(&p.ID,&p.Product,&p.Code,&p.Version,&p.PriceKopecks,&p.Currency,&raw);err!=nil{return nil,err};if err:=json.Unmarshal(raw,&p.Quotas);err!=nil{return nil,err};out=append(out,p)};return out,rows.Err()
}

func (r *Repository) CreatePendingSubscription(ctx context.Context,accountID int64,planCode string,now time.Time)(Subscription,Invoice,error){
	planCode=strings.ToUpper(strings.TrimSpace(planCode));if r==nil||r.db==nil||accountID<=0||planCode==""{return Subscription{},Invoice{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return Subscription{},Invoice{},err};defer func(){_=tx.Rollback(ctx)}()
	var accountType,status string;var plan Plan
	if err=tx.QueryRow(ctx,`SELECT CASE WHEN webmaster_user_id IS NOT NULL THEN 'USER' WHEN agency_id IS NOT NULL THEN 'AGENCY' ELSE 'PLACE' END,status FROM billing_accounts WHERE account_id=$1 FOR UPDATE`,accountID).Scan(&accountType,&status);errors.Is(err,pgx.ErrNoRows){return Subscription{},Invoice{},ErrNotFound};if err!=nil{return Subscription{},Invoice{},err};if status!="ACTIVE"{return Subscription{},Invoice{},ErrForbidden}
	var raw []byte;err=tx.QueryRow(ctx,`SELECT plan_id,product_code,plan_code,version,monthly_price_kopecks,currency,quotas FROM billing_plans WHERE plan_code=$1 AND status='ACTIVE' ORDER BY version DESC LIMIT 1`,planCode).Scan(&plan.ID,&plan.Product,&plan.Code,&plan.Version,&plan.PriceKopecks,&plan.Currency,&raw);if errors.Is(err,pgx.ErrNoRows){return Subscription{},Invoice{},ErrNotFound};if err!=nil{return Subscription{},Invoice{},err};if err=json.Unmarshal(raw,&plan.Quotas);err!=nil{return Subscription{},Invoice{},err}
	if !productAllowedForOwner(plan.Product,accountType){return Subscription{},Invoice{},ErrForbidden}
	periodEnd:=now.AddDate(0,1,0);var sub Subscription
	err=tx.QueryRow(ctx,`INSERT INTO billing_subscriptions(account_id,plan_id,product_code,status,current_period_start,current_period_end) VALUES($1,$2,$3,'PENDING',$4,$5) RETURNING subscription_id,account_id,plan_id,status,current_period_start,current_period_end,grace_until`,accountID,plan.ID,plan.Product,now,periodEnd).Scan(&sub.ID,&sub.AccountID,&sub.PlanID,&sub.Status,&sub.PeriodStart,&sub.PeriodEnd,&sub.GraceUntil);if isUniqueViolation(err){return Subscription{},Invoice{},ErrConflict};if err!=nil{return Subscription{},Invoice{},err}
	external:=fmt.Sprintf("poisk-inv-%d-%d",sub.ID,now.Unix());var inv Invoice
	err=tx.QueryRow(ctx,`INSERT INTO billing_invoices(account_id,subscription_id,external_reference,status,amount_kopecks,currency,period_start,period_end,due_at) VALUES($1,$2,$3,'OPEN',$4,$5,$6,$7,$8) RETURNING invoice_id,account_id,subscription_id,status,amount_kopecks,currency,external_reference`,accountID,sub.ID,external,plan.PriceKopecks,plan.Currency,now,periodEnd,now.Add(24*time.Hour)).Scan(&inv.ID,&inv.AccountID,&inv.SubscriptionID,&inv.Status,&inv.AmountKopecks,&inv.Currency,&inv.ExternalReference);if err!=nil{return Subscription{},Invoice{},err}
	_,err=tx.Exec(ctx,`INSERT INTO billing_ledger_entries(account_id,invoice_id,entry_type,amount_kopecks,currency,idempotency_key,details) VALUES($1,$2,'INVOICE',$3,$4,$5,jsonb_build_object('plan_code',$6,'plan_version',$7))`,accountID,inv.ID,plan.PriceKopecks,plan.Currency,fmt.Sprintf("invoice:%d",inv.ID),plan.Code,plan.Version);if err!=nil{return Subscription{},Invoice{},err}
	if err=tx.Commit(ctx);err!=nil{return Subscription{},Invoice{},err};return sub,inv,nil
}

func productAllowedForOwner(product,owner string)bool{switch product{case "WEBMASTER_PRO","SITE_SEARCH_PRO","BUSINESS_PRO","SEARCH_API","GEO_API":return owner=="USER";case "AGENCY":return owner=="AGENCY"};return false}
func isUniqueViolation(err error)bool{var pgErr *pgconn.PgError;return errors.As(err,&pgErr)&&pgErr.Code=="23505"}

func (r *Repository) Entitlement(ctx context.Context,accountID int64,product string,now time.Time)(Entitlement,error){
	product=strings.ToUpper(strings.TrimSpace(product));if r==nil||r.db==nil||accountID<=0||product==""{return Entitlement{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};var out Entitlement;var raw []byte;var status string;var grace *time.Time
	err:=r.db.QueryRow(ctx,`SELECT p.product_code,p.plan_code,p.version,p.quotas,s.current_period_start,s.current_period_end,s.status,s.grace_until FROM billing_subscriptions s JOIN billing_plans p ON p.plan_id=s.plan_id WHERE s.account_id=$1 AND p.product_code=$2 AND s.status IN ('ACTIVE','GRACE') ORDER BY s.subscription_id DESC LIMIT 1`,accountID,product).Scan(&out.Product,&out.PlanCode,&out.PlanVersion,&raw,&out.PeriodStart,&out.PeriodEnd,&status,&grace);if errors.Is(err,pgx.ErrNoRows){return Entitlement{Product:product},nil};if err!=nil{return Entitlement{},err};if err=json.Unmarshal(raw,&out.Quotas);err!=nil{return Entitlement{},err};out.Active=now.Before(out.PeriodEnd)||(status=="GRACE"&&grace!=nil&&now.Before(*grace));return out,nil
}

func quotaValue(quotas map[string]int64,key string)(int64,bool){v,ok:=quotas[key];return v,ok&&v>=0}

func (r *Repository) Consume(ctx context.Context,accountID int64,product,metric string,amount int64,now time.Time)(used,limit int64,err error){
	metric=strings.ToLower(strings.TrimSpace(metric));if amount<=0||len(metric)<2||len(metric)>64{return 0,0,ErrInvalid};ent,err:=r.Entitlement(ctx,accountID,product,now);if err!=nil{return 0,0,err};if !ent.Active{return 0,0,ErrQuotaExceeded};limit,ok:=quotaValue(ent.Quotas,metric);if !ok{return 0,0,ErrForbidden};if now.IsZero(){now=time.Now().UTC()}
	tx,err:=r.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return 0,limit,err};defer func(){_=tx.Rollback(ctx)}();var current int64
	err=tx.QueryRow(ctx,`INSERT INTO billing_usage_periods(account_id,metric_key,period_start,period_end,used) VALUES($1,$2,$3,$4,$5) ON CONFLICT(account_id,metric_key,period_start,period_end) DO UPDATE SET used=billing_usage_periods.used+EXCLUDED.used,updated_at=now() WHERE billing_usage_periods.used+EXCLUDED.used<=$6 RETURNING used`,accountID,metric,ent.PeriodStart,ent.PeriodEnd,amount,limit).Scan(&current);if errors.Is(err,pgx.ErrNoRows){return 0,limit,ErrQuotaExceeded};if err!=nil{return 0,limit,err};if err=tx.Commit(ctx);err!=nil{return 0,limit,err};return current,limit,nil
}

func hashPayload(payload []byte)[32]byte{return sha256.Sum256(payload)}