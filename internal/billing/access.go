package billing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) AccountForUser(ctx context.Context,userID,accountID int64)(Account,error){
	if r==nil||r.db==nil||userID<=0||accountID<=0{return Account{},ErrInvalid}
	var out Account;var user,agency,place *int64
	err:=r.db.QueryRow(ctx,`SELECT account_id,webmaster_user_id,agency_id,place_id,status FROM billing_accounts WHERE account_id=$1`,accountID).Scan(&out.ID,&user,&agency,&place,&out.Status)
	if errors.Is(err,pgx.ErrNoRows){return Account{},ErrNotFound};if err!=nil{return Account{},err}
	switch{
	case user!=nil:
		if *user!=userID{return Account{},ErrForbidden};out.OwnerType="USER";out.OwnerID=*user
	case agency!=nil:
		var ok bool;if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM agency_members WHERE agency_id=$1 AND user_id=$2 AND status='ACTIVE' AND role='OWNER')`,*agency,userID).Scan(&ok);err!=nil{return Account{},err};if !ok{return Account{},ErrForbidden};out.OwnerType="AGENCY";out.OwnerID=*agency
	case place!=nil:
		var ok bool;if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims WHERE place_id=$1 AND user_id=$2 AND status='ACTIVE')`,*place,userID).Scan(&ok);err!=nil{return Account{},err};if !ok{return Account{},ErrForbidden};out.OwnerType="PLACE";out.OwnerID=*place
	default:return Account{},ErrInvalid
	}
	return out,nil
}

func (r *Repository) Usage(ctx context.Context,userID,accountID int64)(map[string]int64,error){
	if _,err:=r.AccountForUser(ctx,userID,accountID);err!=nil{return nil,err}
	rows,err:=r.db.Query(ctx,`SELECT metric_key,used FROM billing_usage_periods WHERE account_id=$1 AND period_end>now() ORDER BY metric_key`,accountID);if err!=nil{return nil,err};defer rows.Close();out:=map[string]int64{};for rows.Next(){var key string;var used int64;if err:=rows.Scan(&key,&used);err!=nil{return nil,err};out[key]=used};return out,rows.Err()
}
