package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ConsumerUserRow struct {
	ID int64 `json:"user_id"`
	Email string `json:"email"`
	Status string `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	FailedLoginCount int `json:"failed_login_count"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ConsumerSessionRow struct {
	ID int64 `json:"session_id"`
	UserID int64 `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt time.Time `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type ConsumerSecurityEventRow struct {
	ID int64 `json:"event_id"`
	UserID *int64 `json:"user_id,omitempty"`
	EventType string `json:"event_type"`
	CreatedAt time.Time `json:"created_at"`
}

type WebmasterSiteRow struct {
	SiteID int64 `json:"site_id"`
	UserID int64 `json:"user_id"`
	Email string `json:"email"`
	Host string `json:"host"`
	Origin string `json:"origin"`
	Status string `json:"status"`
	VerificationMethod string `json:"verification_method,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

type BillingInvoiceRow struct {
	InvoiceID int64 `json:"invoice_id"`
	AccountID int64 `json:"account_id"`
	SubscriptionID *int64 `json:"subscription_id,omitempty"`
	Status string `json:"status"`
	AmountKopecks int64 `json:"amount_kopecks"`
	Currency string `json:"currency"`
	DueAt *time.Time `json:"due_at,omitempty"`
	PaidAt *time.Time `json:"paid_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s Service) ListConsumerUsers(ctx context.Context,session Session,query string,limit int)([]ConsumerUserRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	query=strings.ToLower(strings.TrimSpace(query));if len(query)>254{return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>100{limit=100}
	rows,err:=s.Store.db.Query(ctx,`SELECT user_id,email,status,email_verified_at,failed_login_count,locked_until,created_at FROM consumer_users WHERE $1='' OR lower(email) LIKE '%'||$1||'%' ORDER BY user_id DESC LIMIT $2`,query,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]ConsumerUserRow,0,limit);for rows.Next(){var row ConsumerUserRow;if err:=rows.Scan(&row.ID,&row.Email,&row.Status,&row.EmailVerifiedAt,&row.FailedLoginCount,&row.LockedUntil,&row.CreatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) ListConsumerSessions(ctx context.Context,session Session,userID int64,limit int)([]ConsumerSessionRow,error){
	if s.Store==nil||s.Store.db==nil||userID<=0{return nil,ErrInvalidCredential}
	if err:=s.RequireRole(session,"OPERATOR");err!=nil{return nil,err};if limit<=0{limit=20};if limit>100{limit=100}
	rows,err:=s.Store.db.Query(ctx,`SELECT session_id,user_id,expires_at,last_seen_at,created_at,revoked_at FROM consumer_sessions WHERE user_id=$1 ORDER BY created_at DESC,session_id DESC LIMIT $2`,userID,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]ConsumerSessionRow,0,limit);for rows.Next(){var row ConsumerSessionRow;if err:=rows.Scan(&row.ID,&row.UserID,&row.ExpiresAt,&row.LastSeenAt,&row.CreatedAt,&row.RevokedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) ListConsumerSecurityEvents(ctx context.Context,session Session,userID int64,limit int)([]ConsumerSecurityEventRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR");err!=nil{return nil,err};if userID<0{return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT event_id,user_id,event_type,created_at FROM consumer_security_events WHERE $1=0 OR user_id=$1 ORDER BY created_at DESC,event_id DESC LIMIT $2`,userID,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]ConsumerSecurityEventRow,0,limit);for rows.Next(){var row ConsumerSecurityEventRow;if err:=rows.Scan(&row.ID,&row.UserID,&row.EventType,&row.CreatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) ListWebmasterSites(ctx context.Context,session Session,query string,limit int)([]WebmasterSiteRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	query=strings.ToLower(strings.TrimSpace(query));if len(query)>254{return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>100{limit=100}
	rows,err:=s.Store.db.Query(ctx,`SELECT s.site_id,s.user_id,u.email,s.host,s.origin,s.status,COALESCE(s.verification_method,''),s.verified_at FROM webmaster_sites s JOIN webmaster_users u ON u.user_id=s.user_id WHERE $1='' OR lower(s.host) LIKE '%'||$1||'%' OR lower(u.email) LIKE '%'||$1||'%' ORDER BY s.site_id DESC LIMIT $2`,query,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]WebmasterSiteRow,0,limit);for rows.Next(){var row WebmasterSiteRow;if err:=rows.Scan(&row.SiteID,&row.UserID,&row.Email,&row.Host,&row.Origin,&row.Status,&row.VerificationMethod,&row.VerifiedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) ListBillingInvoices(ctx context.Context,session Session,status string,limit int)([]BillingInvoiceRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")}
	if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER");err!=nil{return nil,err}
	status=strings.ToUpper(strings.TrimSpace(status));if status!=""&&status!="OPEN"&&status!="PAID"&&status!="VOID"&&status!="FAILED"&&status!="REFUNDED"{return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>100{limit=100}
	rows,err:=s.Store.db.Query(ctx,`SELECT invoice_id,account_id,subscription_id,status,amount_kopecks,currency,due_at,paid_at,created_at FROM billing_invoices WHERE $1='' OR status=$1 ORDER BY created_at DESC,invoice_id DESC LIMIT $2`,status,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]BillingInvoiceRow,0,limit);for rows.Next(){var row BillingInvoiceRow;if err:=rows.Scan(&row.InvoiceID,&row.AccountID,&row.SubscriptionID,&row.Status,&row.AmountKopecks,&row.Currency,&row.DueAt,&row.PaidAt,&row.CreatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}
