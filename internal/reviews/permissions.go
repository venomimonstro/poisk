package reviews

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Permissions struct {
	MyReviewID *int64 `json:"my_review_id,omitempty"`
	CanReply bool `json:"can_reply"`
}

func (r Repository) Permissions(ctx context.Context,userID,placeID int64)(Permissions,error){
	if r.DB==nil||userID<=0||placeID<=0{return Permissions{},ErrInvalid}
	var out Permissions;var reviewID int64
	err:=r.DB.QueryRow(ctx,`SELECT review_id FROM organization_reviews WHERE place_id=$1 AND consumer_user_id=$2`,placeID,userID).Scan(&reviewID)
	if err==nil{out.MyReviewID=&reviewID}else if !errors.Is(err,pgx.ErrNoRows){return Permissions{},err}
	if err:=r.DB.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims c JOIN webmaster_users w ON w.user_id=c.user_id WHERE c.place_id=$1 AND c.status='ACTIVE' AND w.consumer_user_id=$2)`,placeID,userID).Scan(&out.CanReply);err!=nil{return Permissions{},err}
	return out,nil
}
