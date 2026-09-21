package billing

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) ValidateSubscriptionEligibility(ctx context.Context,accountID int64,planCode string)error{
	planCode=strings.ToUpper(strings.TrimSpace(planCode));if r==nil||r.db==nil||accountID<=0||planCode==""{return ErrInvalid}
	var product string
	var userID *int64
	err:=r.db.QueryRow(ctx,`SELECT p.product_code,a.webmaster_user_id
FROM billing_accounts a
JOIN billing_plans p ON p.plan_code=$2 AND p.status='ACTIVE'
WHERE a.account_id=$1 AND a.status='ACTIVE'
ORDER BY p.version DESC LIMIT 1`,accountID,planCode).Scan(&product,&userID)
	if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err}
	if product!="BUSINESS_PRO"{return nil}
	if userID==nil{return ErrForbidden}
	var ok bool
	if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims c JOIN organizations o ON o.place_id=c.place_id WHERE c.user_id=$1 AND c.status='ACTIVE' AND o.status IN ('ACTIVE','REVIEW'))`,*userID).Scan(&ok);err!=nil{return err};if !ok{return ErrForbidden};return nil
}
