package mail

import (
	"context"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid = errors.New("invalid mail input")
	ErrNotFound = errors.New("mail item not found")
	ErrForbidden = errors.New("mail action forbidden")
	ErrConflict = errors.New("mail conflict")
	ErrRateLimited = errors.New("mail rate limited")
)

const (
	maxRecipients = 50
	maxSubjectRunes = 998
	maxBodyRunes = 200000
)

type Repository struct{ DB *pgxpool.Pool }

type Mailbox struct {
	ID int64 `json:"mailbox_id"`
	Address string `json:"address"`
	Status string `json:"status"`
	StorageQuotaBytes int64 `json:"storage_quota_bytes"`
	StorageUsedBytes int64 `json:"storage_used_bytes"`
}

type Recipient struct {
	Address string `json:"address"`
	Type string `json:"type"`
}

type Message struct {
	ID int64 `json:"message_id"`
	ThreadID int64 `json:"thread_id"`
	Subject string `json:"subject"`
	BodyText string `json:"body_text"`
	BodyHTML string `json:"body_html,omitempty"`
	Provenance string `json:"provenance"`
	State string `json:"state"`
	SenderAddress string `json:"sender_address"`
	Recipients []Recipient `json:"recipients,omitempty"`
	SentAt *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Item struct {
	ItemID int64 `json:"mail_item_id"`
	Message Message `json:"message"`
	Folder string `json:"folder"`
	Role string `json:"role"`
	IsRead bool `json:"is_read"`
	IsStarred bool `json:"is_starred"`
	CreatedAt time.Time `json:"created_at"`
}

type DraftInput struct {
	Subject string
	BodyText string
	Recipients []Recipient
	Provenance string
	ParentMessageID *int64
}

func normalizeText(raw string,min,max int)(string,error){
	raw=strings.TrimSpace(raw);n:=utf8.RuneCountInString(raw)
	if n<min||n>max||strings.ContainsRune(raw,'\x00'){return "",ErrInvalid}
	return raw,nil
}

func sanitizeHTMLFromText(raw string) string {
	escaped:=html.EscapeString(raw)
	return strings.ReplaceAll(escaped,"\n","<br>\n")
}

func internalAddress(userID int64) string { return fmt.Sprintf("u%d@internal.poisk",userID) }

func (r Repository) EnsureMailbox(ctx context.Context,userID int64)(Mailbox,error){
	if r.DB==nil||userID<=0{return Mailbox{},ErrInvalid}
	var out Mailbox
	err:=r.DB.QueryRow(ctx,`INSERT INTO mailboxes(consumer_user_id,address)
SELECT user_id,$2 FROM consumer_users WHERE user_id=$1 AND status='ACTIVE'
ON CONFLICT(consumer_user_id) DO UPDATE SET updated_at=mailboxes.updated_at
RETURNING mailbox_id,address,status,storage_quota_bytes,storage_used_bytes`,userID,internalAddress(userID)).Scan(&out.ID,&out.Address,&out.Status,&out.StorageQuotaBytes,&out.StorageUsedBytes)
	if errors.Is(err,pgx.ErrNoRows){return Mailbox{},ErrForbidden};return out,err
}

func normalizeRecipients(items []Recipient)([]Recipient,error){
	if len(items)==0||len(items)>maxRecipients{return nil,ErrInvalid}
	seen:=make(map[string]struct{},len(items));out:=make([]Recipient,0,len(items))
	for _,item:=range items{
		address:=strings.ToLower(strings.TrimSpace(item.Address));kind:=strings.ToUpper(strings.TrimSpace(item.Type));if kind==""{kind="TO"}
		if address==""||len(address)>320||(kind!="TO"&&kind!="CC"&&kind!="BCC"){return nil,ErrInvalid}
		if _,ok:=seen[address];ok{continue};seen[address]=struct{}{};out=append(out,Recipient{Address:address,Type:kind})
	}
	if len(out)==0||len(out)>maxRecipients{return nil,ErrInvalid}
	sort.SliceStable(out,func(i,j int)bool{if out[i].Type==out[j].Type{return out[i].Address<out[j].Address};return out[i].Type<out[j].Type})
	return out,nil
}

func normalizeDraft(in DraftInput)(DraftInput,error){
	var err error
	if in.Subject,err=normalizeText(in.Subject,1,maxSubjectRunes);err!=nil{return DraftInput{},err}
	if in.BodyText,err=normalizeText(in.BodyText,1,maxBodyRunes);err!=nil{return DraftInput{},err}
	if len(in.Recipients)>0{if in.Recipients,err=normalizeRecipients(in.Recipients);err!=nil{return DraftInput{},err}}
	in.Provenance=strings.ToUpper(strings.TrimSpace(in.Provenance));if in.Provenance==""{in.Provenance="COMPOSE"}
	if in.Provenance!="COMPOSE"&&in.Provenance!="REPLY"&&in.Provenance!="FORWARD"{return DraftInput{},ErrInvalid}
	if in.Provenance!="COMPOSE"&&(in.ParentMessageID==nil||*in.ParentMessageID<=0){return DraftInput{},ErrInvalid}
	return in,nil
}

func folderID(ctx context.Context,tx pgx.Tx,mailboxID int64,kind string)(int64,error){var id int64;err:=tx.QueryRow(ctx,`SELECT folder_id FROM mail_folders WHERE mailbox_id=$1 AND kind=$2`,mailboxID,kind).Scan(&id);return id,err}

func (r Repository) CreateDraft(ctx context.Context,userID int64,in DraftInput)(Message,error){
	if r.DB==nil||userID<=0{return Message{},ErrInvalid};var err error;if in,err=normalizeDraft(in);err!=nil{return Message{},err}
	box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return Message{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return Message{},err};defer func(){_=tx.Rollback(ctx)}()
	threadID,parentID,err:=resolveThread(ctx,tx,box.ID,in);if err!=nil{return Message{},err}
	var out Message
	err=tx.QueryRow(ctx,`INSERT INTO mail_messages(thread_id,sender_mailbox_id,parent_message_id,provenance,subject,body_text,body_html,state)
VALUES($1,$2,$3,$4,$5,$6,$7,'DRAFT') RETURNING message_id,thread_id,subject,body_text,COALESCE(body_html,''),provenance,state,sent_at,created_at,updated_at`,threadID,box.ID,parentID,in.Provenance,in.Subject,in.BodyText,sanitizeHTMLFromText(in.BodyText)).Scan(&out.ID,&out.ThreadID,&out.Subject,&out.BodyText,&out.BodyHTML,&out.Provenance,&out.State,&out.SentAt,&out.CreatedAt,&out.UpdatedAt);if err!=nil{return Message{},err}
	if err=replaceRecipients(ctx,tx,out.ID,in.Recipients);err!=nil{return Message{},err}
	draftsID,err:=folderID(ctx,tx,box.ID,"DRAFTS");if err!=nil{return Message{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'DRAFT',$3,TRUE)`,box.ID,out.ID,draftsID);err!=nil{return Message{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type) VALUES($1,$2,'DRAFT_CREATE')`,box.ID,out.ID);err!=nil{return Message{},err}
	out.SenderAddress=box.Address;out.Recipients=in.Recipients
	if err=tx.Commit(ctx);err!=nil{return Message{},err};return out,nil
}

func resolveThread(ctx context.Context,tx pgx.Tx,senderMailboxID int64,in DraftInput)(int64,*int64,error){
	if in.ParentMessageID!=nil{
		var threadID int64;var visible bool
		err:=tx.QueryRow(ctx,`SELECT m.thread_id,EXISTS(SELECT 1 FROM mail_items i WHERE i.message_id=m.message_id AND i.mailbox_id=$2) FROM mail_messages m WHERE m.message_id=$1 AND m.state='SENT'`,*in.ParentMessageID,senderMailboxID).Scan(&threadID,&visible)
		if errors.Is(err,pgx.ErrNoRows){return 0,nil,ErrNotFound};if err!=nil{return 0,nil,err};if !visible{return 0,nil,ErrForbidden};return threadID,in.ParentMessageID,nil
	}
	var threadID int64;if err:=tx.QueryRow(ctx,`INSERT INTO mail_threads(subject) VALUES($1) RETURNING thread_id`,in.Subject).Scan(&threadID);err!=nil{return 0,nil,err};return threadID,nil,nil
}

func replaceRecipients(ctx context.Context,tx pgx.Tx,messageID int64,items []Recipient)error{
	if _,err:=tx.Exec(ctx,`DELETE FROM mail_recipients WHERE message_id=$1`,messageID);err!=nil{return err}
	for i,item:=range items{
		var mailboxID int64;err:=tx.QueryRow(ctx,`SELECT mailbox_id FROM mailboxes WHERE lower(address)=lower($1) AND status='ACTIVE'`,item.Address).Scan(&mailboxID)
		if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err}
		if _,err=tx.Exec(ctx,`INSERT INTO mail_recipients(message_id,recipient_mailbox_id,recipient_type,ordinal) VALUES($1,$2,$3,$4)`,messageID,mailboxID,item.Type,i+1);err!=nil{return err}
	}
	return nil
}

func (r Repository) UpdateDraft(ctx context.Context,userID,messageID int64,in DraftInput)(Message,error){
	if r.DB==nil||userID<=0||messageID<=0{return Message{},ErrInvalid};var err error;if in,err=normalizeDraft(in);err!=nil{return Message{},err};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return Message{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return Message{},err};defer func(){_=tx.Rollback(ctx)}()
	var existingParent *int64;var existingProvenance string
	err=tx.QueryRow(ctx,`SELECT parent_message_id,provenance FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2 AND state='DRAFT' FOR UPDATE`,messageID,box.ID).Scan(&existingParent,&existingProvenance);if errors.Is(err,pgx.ErrNoRows){return Message{},ErrNotFound};if err!=nil{return Message{},err}
	if in.Provenance!=existingProvenance{return Message{},ErrConflict};if (existingParent==nil)!=(in.ParentMessageID==nil)||(existingParent!=nil&&in.ParentMessageID!=nil&&*existingParent!=*in.ParentMessageID){return Message{},ErrConflict}
	var out Message;err=tx.QueryRow(ctx,`UPDATE mail_messages SET subject=$3,body_text=$4,body_html=$5,updated_at=now() WHERE message_id=$1 AND sender_mailbox_id=$2 RETURNING message_id,thread_id,subject,body_text,COALESCE(body_html,''),provenance,state,sent_at,created_at,updated_at`,messageID,box.ID,in.Subject,in.BodyText,sanitizeHTMLFromText(in.BodyText)).Scan(&out.ID,&out.ThreadID,&out.Subject,&out.BodyText,&out.BodyHTML,&out.Provenance,&out.State,&out.SentAt,&out.CreatedAt,&out.UpdatedAt);if err!=nil{return Message{},err}
	if err=replaceRecipients(ctx,tx,messageID,in.Recipients);err!=nil{return Message{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type) VALUES($1,$2,'DRAFT_UPDATE')`,box.ID,messageID);err!=nil{return Message{},err};out.SenderAddress=box.Address;out.Recipients=in.Recipients
	if err=tx.Commit(ctx);err!=nil{return Message{},err};return out,nil
}

func consumeBucket(ctx context.Context,tx pgx.Tx,mailboxID int64,action string,delta,limit int64,now time.Time)error{
	if delta<=0||limit<=0{return ErrInvalid};bucket:=now.UTC().Truncate(time.Hour);var value int64
	err:=tx.QueryRow(ctx,`INSERT INTO mail_action_buckets(mailbox_id,action,bucket_start,value) VALUES($1,$2,$3,$4)
ON CONFLICT(mailbox_id,action,bucket_start) DO UPDATE SET value=mail_action_buckets.value+EXCLUDED.value,updated_at=now()
WHERE mail_action_buckets.value+EXCLUDED.value <= $5 RETURNING value`,mailboxID,action,bucket,delta,limit).Scan(&value)
	if errors.Is(err,pgx.ErrNoRows){return ErrRateLimited};return err
}

func (r Repository) SendDraft(ctx context.Context,userID,messageID int64,now time.Time)(Message,error){
	if r.DB==nil||userID<=0||messageID<=0{return Message{},ErrInvalid};if now.IsZero(){now=time.Now().UTC()};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return Message{},err}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return Message{},err};defer func(){_=tx.Rollback(ctx)}()
	var out Message;err=tx.QueryRow(ctx,`SELECT message_id,thread_id,subject,body_text,COALESCE(body_html,''),provenance,state,sent_at,created_at,updated_at FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2 AND state='DRAFT' FOR UPDATE`,messageID,box.ID).Scan(&out.ID,&out.ThreadID,&out.Subject,&out.BodyText,&out.BodyHTML,&out.Provenance,&out.State,&out.SentAt,&out.CreatedAt,&out.UpdatedAt);if errors.Is(err,pgx.ErrNoRows){return Message{},ErrNotFound};if err!=nil{return Message{},err}
	rows,err:=tx.Query(ctx,`SELECT mb.mailbox_id,mb.address,r.recipient_type,r.ordinal FROM mail_recipients r JOIN mailboxes mb ON mb.mailbox_id=r.recipient_mailbox_id WHERE r.message_id=$1 AND mb.status='ACTIVE' ORDER BY r.recipient_type,r.ordinal`,messageID);if err!=nil{return Message{},err}
	type resolved struct{id int64;address,kind string};recipients:=[]resolved{};for rows.Next(){var x resolved;var ordinal int;if err=rows.Scan(&x.id,&x.address,&x.kind,&ordinal);err!=nil{rows.Close();return Message{},err};recipients=append(recipients,x)};if err=rows.Err();err!=nil{rows.Close();return Message{},err};rows.Close();if len(recipients)==0||len(recipients)>maxRecipients{return Message{},ErrInvalid}
	if err=consumeBucket(ctx,tx,box.ID,"SEND",1,100,now);err!=nil{return Message{},err};if err=consumeBucket(ctx,tx,box.ID,"RECIPIENT",int64(len(recipients)),1000,now);err!=nil{return Message{},err}
	if _,err=tx.Exec(ctx,`UPDATE mail_messages SET state='SENT',sent_at=$2,updated_at=$2 WHERE message_id=$1`,messageID,now);err!=nil{return Message{},err}
	if _,err=tx.Exec(ctx,`DELETE FROM mail_items WHERE mailbox_id=$1 AND message_id=$2 AND item_role='DRAFT'`,box.ID,messageID);err!=nil{return Message{},err}
	sentID,err:=folderID(ctx,tx,box.ID,"SENT");if err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'SENT_COPY',$3,TRUE)`,box.ID,messageID,sentID);err!=nil{return Message{},err}
	for _,recipient:=range recipients{inboxID,e:=folderID(ctx,tx,recipient.id,"INBOX");if e!=nil{return Message{},e};if _,e=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'DELIVERY',$3,FALSE)`,recipient.id,messageID,inboxID);e!=nil{return Message{},e};out.Recipients=append(out.Recipients,Recipient{Address:recipient.address,Type:recipient.kind})}
	if _,err=tx.Exec(ctx,`UPDATE mail_threads SET updated_at=$2 WHERE thread_id=$1`,out.ThreadID,now);err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) VALUES($1,$2,'SEND',jsonb_build_object('recipient_count',$3))`,box.ID,messageID,len(recipients));err!=nil{return Message{},err}
	out.State="SENT";out.SentAt=&now;out.UpdatedAt=now;out.SenderAddress=box.Address;if err=tx.Commit(ctx);err!=nil{return Message{},err};return out,nil
}
