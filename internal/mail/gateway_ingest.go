package mail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type InboundStore struct {
	Repo Repository
	Root string
}

type preparedInboundBlob struct {
	blobID string
	storageKey string
	path string
	filename string
	contentType string
	sha256 string
	bytes int64
}

func (s InboundStore) ResolveRecipient(ctx context.Context,raw string)(int64,error){
	if s.Repo.DB==nil{return 0,ErrInvalid};address,err:=InboundRecipientAddress(raw);if err!=nil{return 0,err}
	alias,err:=s.Repo.ResolveInboundAlias(ctx,address);if err!=nil{return 0,err};return alias.MailboxID,nil
}

func (s InboundStore) Ingest(ctx context.Context,eventID,recipient string,raw []byte,now time.Time)(messageID int64,duplicate bool,err error){
	if s.Repo.DB==nil||strings.TrimSpace(s.Root)==""{return 0,false,ErrInvalid};eventID=strings.TrimSpace(eventID);if len(eventID)<16||len(eventID)>160||strings.ContainsAny(eventID,"\r\n\x00"){return 0,false,ErrInvalid};if now.IsZero(){now=time.Now().UTC()}
	address,err:=InboundRecipientAddress(recipient);if err!=nil{return 0,false,err};mailboxID,err:=s.ResolveRecipient(ctx,address);if err!=nil{return 0,false,err}
	digestBytes:=sha256.Sum256(raw);bodyDigest:=hex.EncodeToString(digestBytes[:])
	var existingDigest,existingRecipient,existingStatus string;var existingMessage *int64
	err=s.Repo.DB.QueryRow(ctx,`SELECT body_sha256,envelope_recipient,status,message_id FROM mail_inbound_receipts WHERE event_id=$1`,eventID).Scan(&existingDigest,&existingRecipient,&existingStatus,&existingMessage)
	if err==nil{if existingDigest!=bodyDigest||!strings.EqualFold(existingRecipient,address){return 0,false,ErrConflict};if existingStatus=="ACCEPTED"&&existingMessage!=nil{return *existingMessage,true,nil};return 0,true,ErrConflict};if !errors.Is(err,pgx.ErrNoRows){return 0,false,err}
	parsed,err:=ParseInboundMIME(raw);if err!=nil{return 0,false,err}
	if err=os.MkdirAll(s.Root,0700);err!=nil{return 0,false,err}
	blobs:=make([]preparedInboundBlob,0,len(parsed.Attachments));cleanup:=func(){for _,b:=range blobs{_ = os.Remove(b.path)}};committed:=false;defer func(){if !committed{cleanup()}}()
	var totalAttachmentBytes int64
	for _,a:=range parsed.Attachments{
		blobID,e:=randomUUID();if e!=nil{return 0,false,e};storageKey,e:=randomUUID();if e!=nil{return 0,false,e};p:=filepath.Join(s.Root,storageKey+".blob")
		if e=os.WriteFile(p,a.Data,0600);e!=nil{return 0,false,e};sum:=sha256.Sum256(a.Data);size:=int64(len(a.Data));totalAttachmentBytes+=size
		blobs=append(blobs,preparedInboundBlob{blobID:blobID,storageKey:storageKey,path:p,filename:a.Filename,contentType:a.ContentType,sha256:hex.EncodeToString(sum[:]),bytes:size})
	}
	tx,err:=s.Repo.DB.Begin(ctx);if err!=nil{return 0,false,err};defer func(){_=tx.Rollback(ctx)}()
	var active bool;var quota,used int64
	if err=tx.QueryRow(ctx,`SELECT status='ACTIVE',storage_quota_bytes,storage_used_bytes FROM mailboxes WHERE mailbox_id=$1 FOR UPDATE`,mailboxID).Scan(&active,&quota,&used);err!=nil{return 0,false,err};if !active{return 0,false,ErrForbidden};if used+totalAttachmentBytes>quota{return 0,false,ErrRateLimited}
	var inserted bool
	err=tx.QueryRow(ctx,`WITH ins AS (INSERT INTO mail_inbound_receipts(event_id,body_sha256,envelope_recipient,mailbox_id,status) VALUES($1,$2,$3,$4,'PROCESSING') ON CONFLICT(event_id) DO NOTHING RETURNING TRUE) SELECT COALESCE((SELECT TRUE FROM ins),FALSE)`,eventID,bodyDigest,address,mailboxID).Scan(&inserted);if err!=nil{return 0,false,err}
	if !inserted{
		var otherDigest,otherRecipient,otherStatus string;var otherMessage *int64;if err=tx.QueryRow(ctx,`SELECT body_sha256,envelope_recipient,status,message_id FROM mail_inbound_receipts WHERE event_id=$1`,eventID).Scan(&otherDigest,&otherRecipient,&otherStatus,&otherMessage);err!=nil{return 0,false,err};if otherDigest!=bodyDigest||!strings.EqualFold(otherRecipient,address){return 0,false,ErrConflict};if otherStatus=="ACCEPTED"&&otherMessage!=nil{if err=tx.Commit(ctx);err!=nil{return 0,false,err};cleanup();committed=true;return *otherMessage,true,nil};return 0,true,ErrConflict
	}
	var threadID int64;if err=tx.QueryRow(ctx,`INSERT INTO mail_threads(subject) VALUES($1) RETURNING thread_id`,parsed.Subject).Scan(&threadID);err!=nil{return 0,false,err}
	if err=tx.QueryRow(ctx,`INSERT INTO mail_messages(thread_id,sender_mailbox_id,sender_external_address,internet_message_id,reply_to_address,received_at,provenance,subject,body_text,body_html,state,sent_at) VALUES($1,NULL,$2,NULLIF($3,''),NULLIF($4,''),$5,'INBOUND',$6,$7,NULL,'SENT',$5) RETURNING message_id`,threadID,parsed.From,parsed.InternetMessageID,parsed.ReplyTo,now,parsed.Subject,parsed.BodyText).Scan(&messageID);err!=nil{return 0,false,err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_recipients(message_id,recipient_mailbox_id,recipient_type,ordinal,delivery_address) VALUES($1,$2,'TO',1,$3)`,messageID,mailboxID,address);err!=nil{return 0,false,err}
	var inboxID int64;if err=tx.QueryRow(ctx,`SELECT folder_id FROM mail_folders WHERE mailbox_id=$1 AND kind='INBOX'`,mailboxID).Scan(&inboxID);err!=nil{return 0,false,err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'DELIVERY',$3,FALSE)`,mailboxID,messageID,inboxID);err!=nil{return 0,false,err}
	for i,b:=range blobs{
		if _,err=tx.Exec(ctx,`INSERT INTO mail_attachment_blobs(blob_id,storage_key,sha256,byte_size,content_type) VALUES($1::uuid,$2::uuid,$3,$4,$5)`,b.blobID,b.storageKey,b.sha256,b.bytes,b.contentType);err!=nil{return 0,false,err}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_attachments(message_id,blob_id,original_filename,ordinal) VALUES($1,$2::uuid,$3,$4)`,messageID,b.blobID,b.filename,i+1);err!=nil{return 0,false,err}
	}
	if totalAttachmentBytes>0{if _,err=tx.Exec(ctx,`UPDATE mailboxes SET storage_used_bytes=storage_used_bytes+$2,updated_at=now() WHERE mailbox_id=$1`,mailboxID,totalAttachmentBytes);err!=nil{return 0,false,err}}
	if _,err=tx.Exec(ctx,`UPDATE mail_inbound_receipts SET status='ACCEPTED',message_id=$2,completed_at=$3 WHERE event_id=$1`,eventID,messageID,now);err!=nil{return 0,false,err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_gateway_events(direction,event_type,message_id,mailbox_id,details) VALUES('INBOUND','ACCEPT',$1,$2,jsonb_build_object('event_id',$3,'attachment_count',$4))`,messageID,mailboxID,eventID,len(blobs));err!=nil{return 0,false,err}
	if err=tx.Commit(ctx);err!=nil{return 0,false,err};committed=true;return messageID,false,nil
}
