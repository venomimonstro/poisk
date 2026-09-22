package reviews

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid=errors.New("invalid review input")
	ErrNotFound=errors.New("review not found")
	ErrForbidden=errors.New("review action forbidden")
	ErrConflict=errors.New("review conflict")
)

type Repository struct{ DB *pgxpool.Pool }

type Review struct{
	ID int64 `json:"review_id"`
	PlaceID int64 `json:"place_id"`
	Rating int `json:"rating"`
	Body string `json:"body"`
	Status string `json:"status,omitempty"`
	Version int64 `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	OwnerReply *Reply `json:"owner_reply,omitempty"`
}
type Reply struct{ID int64 `json:"reply_id"`;Body string `json:"body"`;Version int64 `json:"version"`;UpdatedAt time.Time `json:"updated_at"`}
type Stats struct{Count int64 `json:"review_count"`;Average float64 `json:"average_rating"`}

func normalizeBody(v string,min,max int)(string,error){v=strings.TrimSpace(v);n:=len([]rune(v));if n<min||n>max||strings.ContainsRune(v,'\x00'){return "",ErrInvalid};return v,nil}

func (r Repository) ensurePublicPlace(ctx context.Context,placeID int64)error{
	if r.DB==nil||placeID<=0{return ErrInvalid}
	var status string
	err:=r.DB.QueryRow(ctx,`SELECT status FROM organizations WHERE place_id=$1`,placeID).Scan(&status)
	if errors.Is(err,pgx.ErrNoRows){return ErrNotFound}
	if err!=nil{return err}
	if status!="ACTIVE"{return ErrNotFound}
	return nil
}

func (r Repository) Upsert(ctx context.Context,userID,placeID int64,rating int,body string)(Review,error){
	if r.DB==nil||userID<=0||placeID<=0||rating<1||rating>5{return Review{},ErrInvalid};var err error;if body,err=normalizeBody(body,10,4000);err!=nil{return Review{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return Review{},err};defer func(){_=tx.Rollback(ctx)}()
	var trustedAccount bool
	if err=tx.QueryRow(ctx,`SELECT email_verified_at IS NOT NULL OR created_at <= now()-interval '7 days' FROM consumer_users WHERE user_id=$1 AND status='ACTIVE'`,userID).Scan(&trustedAccount);errors.Is(err,pgx.ErrNoRows){return Review{},ErrForbidden}else if err!=nil{return Review{},err}
	var orgStatus string;if err=tx.QueryRow(ctx,`SELECT status FROM organizations WHERE place_id=$1`,placeID).Scan(&orgStatus);errors.Is(err,pgx.ErrNoRows){return Review{},ErrNotFound}else if err!=nil{return Review{},err};if orgStatus!="ACTIVE"{return Review{},ErrForbidden}
	var claimedByUser bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM organization_claims c JOIN webmaster_users w ON w.user_id=c.user_id WHERE c.place_id=$1 AND c.status='ACTIVE' AND w.consumer_user_id=$2)`,placeID,userID).Scan(&claimedByUser);err!=nil{return Review{},err};if claimedByUser{return Review{},ErrForbidden}
	var existingID int64;var existingStatus string
	err=tx.QueryRow(ctx,`SELECT review_id,status FROM organization_reviews WHERE place_id=$1 AND consumer_user_id=$2 FOR UPDATE`,placeID,userID).Scan(&existingID,&existingStatus)
	if errors.Is(err,pgx.ErrNoRows){
		status:="VISIBLE";reason:="USER_CREATE";if !trustedAccount{status="PENDING";reason="USER_CREATE_TRUST_REVIEW"}
		var out Review;err=tx.QueryRow(ctx,`INSERT INTO organization_reviews(place_id,consumer_user_id,rating,body,status,change_actor_type,change_actor_id,change_reason) VALUES($1,$2,$3,$4,$5,'USER',$2,$6) RETURNING review_id,place_id,rating,body,status,version,created_at,updated_at`,placeID,userID,rating,body,status,reason).Scan(&out.ID,&out.PlaceID,&out.Rating,&out.Body,&out.Status,&out.Version,&out.CreatedAt,&out.UpdatedAt);if err!=nil{return Review{},err};if err=tx.Commit(ctx);err!=nil{return Review{},err};return out,nil
	}
	if err!=nil{return Review{},err}
	newStatus:=existingStatus;reason:="USER_EDIT"
	if !trustedAccount{newStatus="PENDING";reason="USER_EDIT_TRUST_REVIEW"}else{switch existingStatus{case "HIDDEN","REJECTED":newStatus="PENDING";reason="USER_EDIT_AFTER_MODERATION";case "DELETED":newStatus="VISIBLE";reason="USER_RESTORE"}}
	var out Review;err=tx.QueryRow(ctx,`UPDATE organization_reviews SET rating=$3,body=$4,status=$5,change_actor_type='USER',change_actor_id=$2,change_reason=$6 WHERE review_id=$1 AND consumer_user_id=$2 RETURNING review_id,place_id,rating,body,status,version,created_at,updated_at`,existingID,userID,rating,body,newStatus,reason).Scan(&out.ID,&out.PlaceID,&out.Rating,&out.Body,&out.Status,&out.Version,&out.CreatedAt,&out.UpdatedAt);if err!=nil{return Review{},err};if err=tx.Commit(ctx);err!=nil{return Review{},err};return out,nil
}

func (r Repository) SoftDelete(ctx context.Context,userID,reviewID int64)error{
	if r.DB==nil||userID<=0||reviewID<=0{return ErrInvalid};tag,err:=r.DB.Exec(ctx,`UPDATE organization_reviews SET status='DELETED',change_actor_type='USER',change_actor_id=$2,change_reason='USER_DELETE' WHERE review_id=$1 AND consumer_user_id=$2 AND status<>'DELETED'`,reviewID,userID);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};return nil
}

func (r Repository) Report(ctx context.Context,userID,reviewID int64,reason,details string)error{
	if r.DB==nil||userID<=0||reviewID<=0{return ErrInvalid};reason=strings.ToUpper(strings.TrimSpace(reason));switch reason{case "SPAM","ABUSE","FAKE","CONFLICT","OTHER":default:return ErrInvalid};details=strings.TrimSpace(details);if len([]rune(details))>1000{return ErrInvalid}
	var author int64;var status string;err:=r.DB.QueryRow(ctx,`SELECT consumer_user_id,status FROM organization_reviews WHERE review_id=$1`,reviewID).Scan(&author,&status);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err};if author==userID||status!="VISIBLE"{return ErrForbidden}
	_,err=r.DB.Exec(ctx,`INSERT INTO organization_review_reports(review_id,reporter_user_id,reason,details) VALUES($1,$2,$3,NULLIF($4,'')) ON CONFLICT(review_id,reporter_user_id) WHERE status='OPEN' DO NOTHING`,reviewID,userID,reason,details);return err
}

func (r Repository) Reply(ctx context.Context,consumerUserID,reviewID int64,body string)(Reply,error){
	if r.DB==nil||consumerUserID<=0||reviewID<=0{return Reply{},ErrInvalid};var err error;if body,err=normalizeBody(body,2,3000);err!=nil{return Reply{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return Reply{},err};defer func(){_=tx.Rollback(ctx)}();var placeID int64;if err=tx.QueryRow(ctx,`SELECT place_id FROM organization_reviews WHERE review_id=$1 AND status<>'DELETED'`,reviewID).Scan(&placeID);errors.Is(err,pgx.ErrNoRows){return Reply{},ErrNotFound}else if err!=nil{return Reply{},err}
	var claimID int64;err=tx.QueryRow(ctx,`SELECT c.claim_id FROM organization_claims c JOIN webmaster_users w ON w.user_id=c.user_id WHERE c.place_id=$1 AND c.status='ACTIVE' AND w.consumer_user_id=$2`,placeID,consumerUserID).Scan(&claimID);if errors.Is(err,pgx.ErrNoRows){return Reply{},ErrForbidden};if err!=nil{return Reply{},err}
	var out Reply;err=tx.QueryRow(ctx,`INSERT INTO organization_review_replies(review_id,claim_id,owner_consumer_user_id,body,status) VALUES($1,$2,$3,$4,'VISIBLE') ON CONFLICT(review_id) DO UPDATE SET claim_id=EXCLUDED.claim_id,owner_consumer_user_id=EXCLUDED.owner_consumer_user_id,body=EXCLUDED.body,status='VISIBLE',version=organization_review_replies.version+1,updated_at=now() RETURNING reply_id,body,version,updated_at`,reviewID,claimID,consumerUserID,body).Scan(&out.ID,&out.Body,&out.Version,&out.UpdatedAt);if err!=nil{return Reply{},err};if err=tx.Commit(ctx);err!=nil{return Reply{},err};return out,nil
}

func (r Repository) Stats(ctx context.Context,placeID int64)(Stats,error){
	if err:=r.ensurePublicPlace(ctx,placeID);err!=nil{return Stats{},err}
	var out Stats
	err:=r.DB.QueryRow(ctx,`SELECT review_count,average_rating::float8 FROM organization_review_stats WHERE place_id=$1`,placeID).Scan(&out.Count,&out.Average)
	if errors.Is(err,pgx.ErrNoRows){return Stats{},nil}
	return out,err
}

func (r Repository) ListPublic(ctx context.Context,placeID int64,limit int,beforeID int64)([]Review,error){
	if r.DB==nil||placeID<=0{return nil,ErrInvalid};if limit<=0{limit=20};if limit>100{limit=100};if beforeID<0{return nil,ErrInvalid}
	if err:=r.ensurePublicPlace(ctx,placeID);err!=nil{return nil,err}
	rows,err:=r.DB.Query(ctx,`SELECT r.review_id,r.place_id,r.rating,r.body,r.version,r.created_at,r.updated_at,rep.reply_id,rep.body,rep.version,rep.updated_at FROM organization_reviews r LEFT JOIN organization_review_replies rep ON rep.review_id=r.review_id AND rep.status='VISIBLE' WHERE r.place_id=$1 AND r.status='VISIBLE' AND ($2=0 OR r.review_id<$2) ORDER BY r.review_id DESC LIMIT $3`,placeID,beforeID,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]Review,0,limit)
	for rows.Next(){var item Review;var replyID *int64;var replyBody *string;var replyVersion *int64;var replyUpdated *time.Time;if err:=rows.Scan(&item.ID,&item.PlaceID,&item.Rating,&item.Body,&item.Version,&item.CreatedAt,&item.UpdatedAt,&replyID,&replyBody,&replyVersion,&replyUpdated);err!=nil{return nil,err};if replyID!=nil&&replyBody!=nil&&replyVersion!=nil&&replyUpdated!=nil{item.OwnerReply=&Reply{ID:*replyID,Body:*replyBody,Version:*replyVersion,UpdatedAt:*replyUpdated}};out=append(out,item)};return out,rows.Err()
}
