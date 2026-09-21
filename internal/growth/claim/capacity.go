package claim

import "context"

func (r *Repository) ActiveCount(ctx context.Context,userID int64)(int64,error){
	if r==nil||r.db==nil||userID<=0{return 0,ErrInvalid}
	var count int64
	err:=r.db.QueryRow(ctx,`SELECT count(*) FROM organization_claims WHERE user_id=$1 AND status='ACTIVE'`,userID).Scan(&count)
	return count,err
}

func (r *Repository) HasActive(ctx context.Context,userID,placeID int64)(bool,error){
	if r==nil||r.db==nil||userID<=0||placeID<=0{return false,ErrInvalid}
	var exists bool
	err:=r.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims WHERE user_id=$1 AND place_id=$2 AND status='ACTIVE')`,userID,placeID).Scan(&exists)
	return exists,err
}
