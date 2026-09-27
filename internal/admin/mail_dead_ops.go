package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

type MailDeadRetryPreview struct {
	Token string `json:"preview_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Delivery mailcore.DeadLetter `json:"delivery"`
}

func (s Service) ListMailDeadLetters(ctx context.Context,session Session,limit int,beforeID int64)([]mailcore.DeadLetter,error){
	if s.Store==nil||s.Store.db==nil{return nil,errors.New("admin service is not initialized")};if err:=s.RequireRole(session,"OPERATOR","ANALYST","VIEWER","SUPPORT");err!=nil{return nil,err}
	return (mailcore.Repository{DB:s.Store.db}).ListDeadLetters(ctx,limit,beforeID)
}

func (s Service) PreviewMailDeadRetry(ctx context.Context,session Session,deliveryID int64)(MailDeadRetryPreview,error){
	if s.Store==nil||s.Store.db==nil||deliveryID<=0{return MailDeadRetryPreview{},ErrPreviewInvalid};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return MailDeadRetryPreview{},err}
	var item mailcore.DeadLetter;var status string
	err:=s.Store.db.QueryRow(ctx,`SELECT delivery_id,message_id,sender_mailbox_id,attempts,COALESCE(last_error_code,''),updated_at,status FROM mail_outbound_deliveries WHERE delivery_id=$1`,deliveryID).Scan(&item.DeliveryID,&item.MessageID,&item.SenderMailboxID,&item.Attempts,&item.ErrorCode,&item.UpdatedAt,&status)
	if errors.Is(err,pgx.ErrNoRows){return MailDeadRetryPreview{},ErrPreviewInvalid};if err!=nil{return MailDeadRetryPreview{},err};item.Retryable=status=="DEAD"&&mailcore.RetryableDeadCode(item.ErrorCode);if !item.Retryable{return MailDeadRetryPreview{},ErrForbidden}
	token,hash,err:=RandomToken(24);if err!=nil{return MailDeadRetryPreview{},err};expires:=time.Now().UTC().Add(5*time.Minute)
	payload,_:=json.Marshal(map[string]any{"delivery_id":deliveryID,"error_code":item.ErrorCode,"updated_at":item.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	_,err=s.Store.db.Exec(ctx,`INSERT INTO admin_action_previews(admin_id,session_id,token_hash,action_type,target_type,target_id,payload,expires_at) VALUES($1,$2,$3,'MAIL_DEAD_RETRY','MAIL_DELIVERY',$4,$5::jsonb,$6)`,session.AdminID,session.ID,hash,deliveryID,string(payload),expires);if err!=nil{return MailDeadRetryPreview{},err}
	_ = s.Store.SecurityEvent(ctx,&session.AdminID,"MAIL_DEAD_RETRY_PREVIEW",true,string(payload));return MailDeadRetryPreview{Token:token,ExpiresAt:expires,Delivery:item},nil
}

func (s Service) ApplyMailDeadRetry(ctx context.Context,session Session,token string)error{
	if s.Store==nil||s.Store.db==nil||strings.TrimSpace(token)==""{return ErrPreviewInvalid};if err:=s.RequireRole(session,"OPERATOR");err!=nil{return err}
	hash:=HashToken(token);tx,err:=s.Store.db.BeginTx(ctx,pgx.TxOptions{});if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var previewID,deliveryID int64;var payloadRaw []byte
	err=tx.QueryRow(ctx,`SELECT preview_id,target_id,payload FROM admin_action_previews WHERE admin_id=$1 AND session_id=$2 AND token_hash=$3 AND action_type='MAIL_DEAD_RETRY' AND target_type='MAIL_DELIVERY' AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`,session.AdminID,session.ID,hash).Scan(&previewID,&deliveryID,&payloadRaw)
	if errors.Is(err,pgx.ErrNoRows){return ErrPreviewInvalid};if err!=nil{return err}
	var payload struct{ErrorCode string `json:"error_code"`;UpdatedAt string `json:"updated_at"`};if err=json.Unmarshal(payloadRaw,&payload);err!=nil||!mailcore.RetryableDeadCode(payload.ErrorCode){return ErrPreviewInvalid}
	var currentCode,status string;var currentUpdated time.Time
	if err=tx.QueryRow(ctx,`SELECT status,COALESCE(last_error_code,''),updated_at FROM mail_outbound_deliveries WHERE delivery_id=$1`,deliveryID).Scan(&status,&currentCode,&currentUpdated);errors.Is(err,pgx.ErrNoRows){return ErrPreviewInvalid}else if err!=nil{return err}
	previewUpdated,parseErr:=time.Parse(time.RFC3339Nano,payload.UpdatedAt);if parseErr!=nil||status!="DEAD"||currentCode!=payload.ErrorCode||!currentUpdated.Equal(previewUpdated){return ErrPreviewInvalid}
	actor:=fmt.Sprintf("admin:%d",session.AdminID);if err=(mailcore.Repository{DB:s.Store.db}).RetryDeadLetter(ctx,deliveryID,actor,time.Now().UTC());err!=nil{return err}
	if _,err=tx.Exec(ctx,`UPDATE admin_action_previews SET consumed_at=now() WHERE preview_id=$1`,previewID);err!=nil{return err}
	details,_:=json.Marshal(map[string]any{"delivery_id":deliveryID,"previous_code":payload.ErrorCode,"preview_id":previewID})
	if _,err=tx.Exec(ctx,`INSERT INTO audit_log(actor_type,actor_id,action,entity_type,entity_id,details) VALUES('ADMIN',$1::bigint::text,'MAIL_DEAD_RETRY_APPLY','MAIL_DELIVERY',$2::bigint::text,$3::jsonb)`,session.AdminID,deliveryID,string(details));err!=nil{return err}
	if err=tx.Commit(ctx);err!=nil{return err};_ = s.Store.SecurityEvent(ctx,&session.AdminID,"MAIL_DEAD_RETRY_APPLY",true,string(details));return nil
}
