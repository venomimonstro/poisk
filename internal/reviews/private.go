package reviews

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (r Repository) MyReview(ctx context.Context,userID,placeID int64)(*Review,error){
	if r.DB==nil||userID<=0||placeID<=0{return nil,ErrInvalid}
	var out Review
	err:=r.DB.QueryRow(ctx,`SELECT review_id,place_id,rating,body,status,version,created_at,updated_at FROM organization_reviews WHERE consumer_user_id=$1 AND place_id=$2`,userID,placeID).Scan(&out.ID,&out.PlaceID,&out.Rating,&out.Body,&out.Status,&out.Version,&out.CreatedAt,&out.UpdatedAt)
	if errors.Is(err,pgx.ErrNoRows){return nil,nil};if err!=nil{return nil,err};return &out,nil
}
