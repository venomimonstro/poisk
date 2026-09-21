package billing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// EnsureAgencySystem is for trusted in-process product enforcement only. It does
// not authorize a user; public handlers must use EnsureAgencyAccount instead.
func (r *Repository) EnsureAgencySystem(ctx context.Context,agencyID int64)(int64,error){
	if r==nil||r.db==nil||agencyID<=0{return 0,ErrInvalid}
	var active bool
	if err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM agencies WHERE agency_id=$1 AND status='ACTIVE')`,agencyID).Scan(&active);err!=nil{return 0,err};if !active{return 0,ErrNotFound}
	var id int64;err:=r.db.QueryRow(ctx,`INSERT INTO billing_accounts(agency_id) VALUES($1)
ON CONFLICT(agency_id) WHERE agency_id IS NOT NULL DO UPDATE SET updated_at=now()
RETURNING account_id`,agencyID).Scan(&id);return id,err
}

func (r *Repository) AccountIDForPlace(ctx context.Context,placeID int64)(int64,error){
	if r==nil||r.db==nil||placeID<=0{return 0,ErrInvalid};var id int64;err:=r.db.QueryRow(ctx,`SELECT account_id FROM billing_accounts WHERE place_id=$1 AND status='ACTIVE'`,placeID).Scan(&id);if errors.Is(err,pgx.ErrNoRows){return 0,ErrNotFound};return id,err
}
