package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReviewModerationRow struct {
	ReviewID int64 `json:"review_id"`
	PlaceID int64 `json:"place_id"`
	PlaceName string `json:"place_name"`
	Rating int `json:"rating"`
	Body string `json:"body"`
	Status string `json:"status"`
	Version int64 `json:"version"`
	OpenReports int64 `json:"open_reports"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReviewModerationPreview struct {
	Token string `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Review ReviewModerationRow `json:"review"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func validReviewModerationAction(v string)bool{switch v{case "HIDE","SHOW","REJECT":return true};return false}

func (s Service) ListReviewModeration(ctx context.Context,session Session,status string,limit int)([]ReviewModerationRow,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	status=strings.ToUpper(strings.TrimSpace(status));switch status{case "","VISIBLE","PENDING","HIDDEN","REJECTED","DELETED":default:return nil,ErrInvalidCredential};if limit<=0{limit=50};if limit>200{limit=200}
	rows,err:=s.Store.db.Query(ctx,`SELECT r.review_id,r.place_id,o.name,r.rating,r.body,r.status,r.version,(SELECT count(*) FROM organization_review_reports rp WHERE rp.review_id=r.review_id AND rp.status='OPEN'),r.updated_at
FROM organization_reviews r JOIN organizations o ON o.place_id=r.place_id
WHERE ($1='' OR r.status=$1) AND ($1<>'' OR r.status='PENDING' OR EXISTS(SELECT 1 FROM organization_review_reports rp WHERE rp.review_id=r.review_id AND rp.status='OPEN'))
ORDER BY CASE WHEN r.status='PENDING' THEN 0 ELSE 1 END,(SELECT count(*) FROM organization_review_reports rp WHERE rp.review_id=r.review_id AND rp.status='OPEN') DESC,r.updated_at,r.review_id LIMIT $2`,status,limit);if err!=nil{return nil,err};defer rows.Close();out:=make([]ReviewModerationRow,0,limit);for rows.Next(){var row ReviewModerationRow;if err:=rows.Scan(&row.ReviewID,&row.PlaceID,&row.PlaceName,&row.Rating,&row.Body,&row.Status,&row.Version,&row.OpenReports,&row.UpdatedAt);err!=nil{return nil,err};out=append(out,row)};return out,rows.Err()
}

func (s Service) PreviewReviewModeration(ctx context.Context,session Session,reviewID int64,action,reason string)(ReviewModerationPreview,error){
	if s.Store==nil||s.Store.db==nil||reviewID<=0{return ReviewModerationPreview{},ErrInvalidCredential};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return ReviewModerationPreview{},err};action=strings.ToUpper(strings.TrimSpace(action));reason=strings.TrimSpace(reason);if !validReviewModerationAction(action)||len([]rune(reason))<2||len([]rune(reason))>240{return ReviewModerationPreview{},ErrInvalidCredential}
	var row ReviewModerationRow;err:=s.Store.db.QueryRow(ctx,`SELECT r.review_id,r.place_id,o.name,r.rating,r.body,r.status,r.version,(SELECT count(*) FROM organization_review_reports rp WHERE rp.review_id=r.review_id AND rp.status='OPEN'),r.updated_at FROM organization_reviews r JOIN organizations o ON o.place_id=r.place_id WHERE r.review_id=$1`,reviewID).Scan(&row.ReviewID,&row.PlaceID,&row.PlaceName,&row.Rating,&row.Body,&row.Status,&row.Version,&row.OpenReports,&row.UpdatedAt);if errors.Is(err,pgx.ErrNoRows){return ReviewModerationPreview{},ErrNotFound};if err!=nil{return ReviewModerationPreview{},err};if row.Status=="DELETED"{return ReviewModerationPreview{},ErrPreviewInvalid}
	token,hash,err:=RandomToken(24);if err!=nil{return ReviewModerationPreview{},err};expires:=time.Now().UTC().Add(5*time.Minute);payload,_:=json.Marshal(map[string]any{"action":action,"reason":reason,"before_status":row.Status,"before_version":row.Version,"place_id":row.PlaceID})
	if _,err=s.Store.db.Exec(ctx,`INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at) VALUES($1,$2,$3,'REVIEW_MODERATION','ORGANIZATION_REVIEW',$4,$5::jsonb,$6)`,session.AdminID,session.ID,hash,reviewID,string(payload),expires);err!=nil{return ReviewModerationPreview{},err};_ = s.Store.SecurityEvent(ctx,&session.AdminID,"REVIEW_MODERATION_PREVIEW",true,string(payload));return ReviewModerationPreview{Token:token,ExpiresAt:expires,Review:row,Action:action,Reason:reason},nil
}

func (s Service) ApplyReviewModeration(ctx context.Context,session Session,token string)(ReviewModerationRow,error){
	if s.Store==nil||s.Store.db==nil||strings.TrimSpace(token)==""{return ReviewModerationRow{},ErrPreviewInvalid};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return ReviewModerationRow{},err}
	tx,err:=s.Store.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return ReviewModerationRow{},err};defer func(){_=tx.Rollback(ctx)}();var previewID,reviewID int64;var raw []byte
	err=tx.QueryRow(ctx,`SELECT preview_id,target_id,payload FROM admin_action_previews WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='REVIEW_MODERATION' AND target_type='ORGANIZATION_REVIEW' AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`,session.AdminID,session.ID,HashToken(token)).Scan(&previewID,&reviewID,&raw);if errors.Is(err,pgx.ErrNoRows){return ReviewModerationRow{},ErrPreviewInvalid};if err!=nil{return ReviewModerationRow{},err}
	var p struct{Action string `json:"action"`;Reason string `json:"reason"`;BeforeStatus string `json:"before_status"`;BeforeVersion int64 `json:"before_version"`;PlaceID int64 `json:"place_id"`};if err:=json.Unmarshal(raw,&p);err!=nil||!validReviewModerationAction(p.Action){return ReviewModerationRow{},ErrPreviewInvalid}
	var currentStatus string;var currentVersion int64;var placeID int64;if err:=tx.QueryRow(ctx,`SELECT status,version,place_id FROM organization_reviews WHERE review_id=$1 FOR UPDATE`,reviewID).Scan(&currentStatus,&currentVersion,&placeID);errors.Is(err,pgx.ErrNoRows){return ReviewModerationRow{},ErrNotFound}else if err!=nil{return ReviewModerationRow{},err};if currentVersion!=p.BeforeVersion||currentStatus!=p.BeforeStatus||placeID!=p.PlaceID{return ReviewModerationRow{},ErrPreviewInvalid}
	nextStatus:="VISIBLE";eventAction:="SHOW";reportStatus:="DISMISSED";reportEvent:="REPORT_DISMISS";if p.Action=="HIDE"{nextStatus="HIDDEN";eventAction="HIDE";reportStatus="RESOLVED";reportEvent="REPORT_RESOLVE"};if p.Action=="REJECT"{nextStatus="REJECTED";eventAction="REJECT";reportStatus="RESOLVED";reportEvent="REPORT_RESOLVE"}
	if _,err=tx.Exec(ctx,`UPDATE organization_reviews SET status=$2,change_actor_type='ADMIN',change_actor_id=$3,change_reason=$4 WHERE review_id=$1`,reviewID,nextStatus,session.AdminID,"ADMIN_"+p.Action);err!=nil{return ReviewModerationRow{},err}
	reportsTag,err:=tx.Exec(ctx,`UPDATE organization_review_reports SET status=$2,resolved_at=now(),resolved_by_admin_id=$3 WHERE review_id=$1 AND status='OPEN'`,reviewID,reportStatus,session.AdminID);if err!=nil{return ReviewModerationRow{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO organization_review_moderation_events(review_id,admin_id,action,reason,details) VALUES($1,$2,$3,$4,jsonb_build_object('preview_id',$5,'from_status',$6,'to_status',$7))`,reviewID,session.AdminID,eventAction,p.Reason,previewID,currentStatus,nextStatus);err!=nil{return ReviewModerationRow{},err}
	if reportsTag.RowsAffected()>0{if _,err=tx.Exec(ctx,`INSERT INTO organization_review_moderation_events(review_id,admin_id,action,reason,details) VALUES($1,$2,$3,$4,jsonb_build_object('preview_id',$5,'reports_affected',$6))`,reviewID,session.AdminID,reportEvent,p.Reason,previewID,reportsTag.RowsAffected());err!=nil{return ReviewModerationRow{},err}}
	if _,err=tx.Exec(ctx,`UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`,previewID);err!=nil{return ReviewModerationRow{},err};details,_:=json.Marshal(map[string]any{"action":p.Action,"reason":p.Reason,"preview_id":previewID,"from_status":currentStatus,"to_status":nextStatus,"reports_affected":reportsTag.RowsAffected()});if _,err=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details) VALUES('ADMIN',$1::bigint::text,'REVIEW_MODERATION_APPLY','ORGANIZATION_REVIEW',$2::bigint::text,$3::jsonb)`,session.AdminID,reviewID,string(details));err!=nil{return ReviewModerationRow{},err}
	var out ReviewModerationRow;if err=tx.QueryRow(ctx,`SELECT r.review_id,r.place_id,o.name,r.rating,r.body,r.status,r.version,(SELECT count(*) FROM organization_review_reports rp WHERE rp.review_id=r.review_id AND rp.status='OPEN'),r.updated_at FROM organization_reviews r JOIN organizations o ON o.place_id=r.place_id WHERE r.review_id=$1`,reviewID).Scan(&out.ReviewID,&out.PlaceID,&out.PlaceName,&out.Rating,&out.Body,&out.Status,&out.Version,&out.OpenReports,&out.UpdatedAt);err!=nil{return ReviewModerationRow{},err};if err=tx.Commit(ctx);err!=nil{return ReviewModerationRow{},err};_ = s.Store.SecurityEvent(ctx,&session.AdminID,"REVIEW_MODERATION_APPLY",true,string(details));return out,nil
}
