package mail

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type DraftEdit struct {
	MessageID int64 `json:"message_id"`
	ThreadID int64 `json:"thread_id"`
	ParentMessageID *int64 `json:"parent_message_id,omitempty"`
	Provenance string `json:"provenance"`
	Subject string `json:"subject"`
	BodyText string `json:"body_text"`
	Recipients []Recipient `json:"recipients"`
}

func (r Repository) GetDraftEdit(ctx context.Context,userID,messageID int64)(DraftEdit,error){
	if r.DB==nil||userID<=0||messageID<=0{return DraftEdit{},ErrInvalid}
	box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return DraftEdit{},err}
	var out DraftEdit
	err=r.DB.QueryRow(ctx,`SELECT message_id,thread_id,parent_message_id,provenance,subject,body_text FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2 AND state='DRAFT'`,messageID,box.ID).Scan(&out.MessageID,&out.ThreadID,&out.ParentMessageID,&out.Provenance,&out.Subject,&out.BodyText)
	if errors.Is(err,pgx.ErrNoRows){return DraftEdit{},ErrNotFound};if err!=nil{return DraftEdit{},err}
	out.Recipients,err=r.messageRecipients(ctx,messageID,box.ID);if err!=nil{return DraftEdit{},err}
	return out,nil
}
