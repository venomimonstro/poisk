package mail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var folderKinds=map[string]bool{"INBOX":true,"SENT":true,"DRAFTS":true,"TRASH":true,"SPAM":true}

func normalizeFolder(kind string)(string,error){kind=strings.ToUpper(strings.TrimSpace(kind));if !folderKinds[kind]{return "",ErrInvalid};return kind,nil}

func (r Repository) ListFolder(ctx context.Context,userID int64,kind string,limit int,beforeID int64)([]Item,error){
	if r.DB==nil||userID<=0||beforeID<0{return nil,ErrInvalid};var err error;if kind,err=normalizeFolder(kind);err!=nil{return nil,err};if limit<=0{limit=50};if limit>100{limit=100}
	box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return nil,err}
	rows,err:=r.DB.Query(ctx,`SELECT i.mail_item_id,i.item_role,i.is_read,i.is_starred,i.created_at,m.message_id,m.thread_id,m.subject,m.body_text,COALESCE(m.body_html,''),m.provenance,m.state,s.address,m.sent_at,m.created_at,m.updated_at
FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id AND f.mailbox_id=i.mailbox_id JOIN mail_messages m ON m.message_id=i.message_id JOIN mailboxes s ON s.mailbox_id=m.sender_mailbox_id
WHERE i.mailbox_id=$1 AND f.kind=$2 AND ($3=0 OR i.mail_item_id<$3)
ORDER BY i.mail_item_id DESC LIMIT $4`,box.ID,kind,beforeID,limit);if err!=nil{return nil,err};defer rows.Close()
	out:=make([]Item,0,limit);for rows.Next(){var x Item;if err:=rows.Scan(&x.ItemID,&x.Role,&x.IsRead,&x.IsStarred,&x.CreatedAt,&x.Message.ID,&x.Message.ThreadID,&x.Message.Subject,&x.Message.BodyText,&x.Message.BodyHTML,&x.Message.Provenance,&x.Message.State,&x.Message.SenderAddress,&x.Message.SentAt,&x.Message.CreatedAt,&x.Message.UpdatedAt);err!=nil{return nil,err};x.Folder=kind;out=append(out,x)};return out,rows.Err()
}

func (r Repository) GetItem(ctx context.Context,userID,itemID int64)(Item,error){
	if r.DB==nil||userID<=0||itemID<=0{return Item{},ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return Item{},err};var x Item
	err=r.DB.QueryRow(ctx,`SELECT i.mail_item_id,f.kind,i.item_role,i.is_read,i.is_starred,i.created_at,m.message_id,m.thread_id,m.subject,m.body_text,COALESCE(m.body_html,''),m.provenance,m.state,s.address,m.sent_at,m.created_at,m.updated_at
FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id AND f.mailbox_id=i.mailbox_id JOIN mail_messages m ON m.message_id=i.message_id JOIN mailboxes s ON s.mailbox_id=m.sender_mailbox_id
WHERE i.mail_item_id=$1 AND i.mailbox_id=$2`,itemID,box.ID).Scan(&x.ItemID,&x.Folder,&x.Role,&x.IsRead,&x.IsStarred,&x.CreatedAt,&x.Message.ID,&x.Message.ThreadID,&x.Message.Subject,&x.Message.BodyText,&x.Message.BodyHTML,&x.Message.Provenance,&x.Message.State,&x.Message.SenderAddress,&x.Message.SentAt,&x.Message.CreatedAt,&x.Message.UpdatedAt)
	if errors.Is(err,pgx.ErrNoRows){return Item{},ErrNotFound};if err!=nil{return Item{},err};recipients,err:=r.messageRecipients(ctx,x.Message.ID,box.ID);if err!=nil{return Item{},err};x.Message.Recipients=recipients;return x,nil
}

func (r Repository) messageRecipients(ctx context.Context,messageID,viewerMailboxID int64)([]Recipient,error){
	rows,err:=r.DB.Query(ctx,`SELECT address,recipient_type,ordinal FROM (
 SELECT r.delivery_address AS address,r.recipient_type,r.ordinal
 FROM mail_recipients r
 WHERE r.message_id=$1 AND (r.recipient_type<>'BCC' OR r.recipient_mailbox_id=$2 OR EXISTS(SELECT 1 FROM mail_messages m WHERE m.message_id=$1 AND m.sender_mailbox_id=$2))
 UNION ALL
 SELECT e.address,e.recipient_type,e.ordinal
 FROM mail_external_recipients e
 WHERE e.message_id=$1 AND (e.recipient_type<>'BCC' OR EXISTS(SELECT 1 FROM mail_messages m WHERE m.message_id=$1 AND m.sender_mailbox_id=$2))
) recipients ORDER BY recipient_type,ordinal,address`,messageID,viewerMailboxID);if err!=nil{return nil,err};defer rows.Close();out:=[]Recipient{};for rows.Next(){var x Recipient;var ordinal int;if err:=rows.Scan(&x.Address,&x.Type,&ordinal);err!=nil{return nil,err};out=append(out,x)};return out,rows.Err()
}

func (r Repository) SetRead(ctx context.Context,userID,itemID int64,read bool)error{
	if r.DB==nil||userID<=0||itemID<=0{return ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return err};tag,err:=r.DB.Exec(ctx,`UPDATE mail_items SET is_read=$3,updated_at=now() WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,read);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};event:="READ";_,err=r.DB.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) SELECT mailbox_id,message_id,$3,jsonb_build_object('read',$4) FROM mail_items WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,event,read);return err
}

func (r Repository) SetStarred(ctx context.Context,userID,itemID int64,starred bool)error{
	if r.DB==nil||userID<=0||itemID<=0{return ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return err};tag,err:=r.DB.Exec(ctx,`UPDATE mail_items SET is_starred=$3,updated_at=now() WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,starred);if err!=nil{return err};if tag.RowsAffected()!=1{return ErrNotFound};_,err=r.DB.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) SELECT mailbox_id,message_id,'STAR',jsonb_build_object('starred',$3) FROM mail_items WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,starred);return err
}

func (r Repository) MoveToTrash(ctx context.Context,userID,itemID int64)error{
	if r.DB==nil||userID<=0||itemID<=0{return ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return err};tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var current string;var messageID int64
	err=tx.QueryRow(ctx,`SELECT f.kind,i.message_id FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id WHERE i.mail_item_id=$1 AND i.mailbox_id=$2 FOR UPDATE OF i`,itemID,box.ID).Scan(&current,&messageID);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err};if current=="TRASH"{return tx.Commit(ctx)};if current!="INBOX"&&current!="SENT"&&current!="DRAFTS"&&current!="SPAM"{return ErrConflict};trashID,err:=folderID(ctx,tx,box.ID,"TRASH");if err!=nil{return err};if _,err=tx.Exec(ctx,`UPDATE mail_items SET folder_id=$3,trashed_from_kind=$4,deleted_at=now(),updated_at=now() WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,trashID,current);err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) VALUES($1,$2,'TRASH',jsonb_build_object('from',$3))`,box.ID,messageID,current);err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) Restore(ctx context.Context,userID,itemID int64)error{
	if r.DB==nil||userID<=0||itemID<=0{return ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return err};tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var from *string;var messageID int64;var current string
	err=tx.QueryRow(ctx,`SELECT i.trashed_from_kind,i.message_id,f.kind FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id WHERE i.mail_item_id=$1 AND i.mailbox_id=$2 FOR UPDATE OF i`,itemID,box.ID).Scan(&from,&messageID,&current);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err};if current!="TRASH"||from==nil{return ErrConflict};targetID,err:=folderID(ctx,tx,box.ID,*from);if err!=nil{return err};if _,err=tx.Exec(ctx,`UPDATE mail_items SET folder_id=$3,trashed_from_kind=NULL,deleted_at=NULL,updated_at=now() WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,targetID);err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) VALUES($1,$2,'RESTORE',jsonb_build_object('to',$3))`,box.ID,messageID,*from);err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) MoveSpam(ctx context.Context,userID,itemID int64,spam bool)error{
	if r.DB==nil||userID<=0||itemID<=0{return ErrInvalid};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return err};tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}();var role,current string;var messageID int64
	err=tx.QueryRow(ctx,`SELECT i.item_role,f.kind,i.message_id FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id WHERE i.mail_item_id=$1 AND i.mailbox_id=$2 FOR UPDATE OF i`,itemID,box.ID).Scan(&role,&current,&messageID);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err};if role!="DELIVERY"{return ErrForbidden};target:="SPAM";event:="SPAM";if !spam{target="INBOX";event="UNSPAM"};if current==target{return tx.Commit(ctx)};if spam&&current!="INBOX"{return ErrConflict};if !spam&&current!="SPAM"{return ErrConflict};targetID,err:=folderID(ctx,tx,box.ID,target);if err!=nil{return err};if _,err=tx.Exec(ctx,`UPDATE mail_items SET folder_id=$3,updated_at=now() WHERE mail_item_id=$1 AND mailbox_id=$2`,itemID,box.ID,targetID);err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type) VALUES($1,$2,$3)`,box.ID,messageID,event);if err!=nil{return err};return tx.Commit(ctx)
}

func (r Repository) Search(ctx context.Context,userID int64,query string,limit int)([]Item,error){
	if r.DB==nil||userID<=0{return nil,ErrInvalid};query=strings.TrimSpace(query);if query==""||len([]rune(query))>256{return nil,ErrInvalid};if limit<=0{limit=50};if limit>100{limit=100};box,err:=r.EnsureMailbox(ctx,userID);if err!=nil{return nil,err}
	rows,err:=r.DB.Query(ctx,`SELECT i.mail_item_id,f.kind,i.item_role,i.is_read,i.is_starred,i.created_at,m.message_id,m.thread_id,m.subject,m.body_text,COALESCE(m.body_html,''),m.provenance,m.state,s.address,m.sent_at,m.created_at,m.updated_at
FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id AND f.mailbox_id=i.mailbox_id JOIN mail_messages m ON m.message_id=i.message_id JOIN mailboxes s ON s.mailbox_id=m.sender_mailbox_id
WHERE i.mailbox_id=$1 AND f.kind<>'TRASH' AND m.search_vector @@ websearch_to_tsquery('simple',$2)
ORDER BY ts_rank_cd(m.search_vector,websearch_to_tsquery('simple',$2)) DESC,i.mail_item_id DESC LIMIT $3`,box.ID,query,limit);if err!=nil{return nil,err};defer rows.Close();out:=[]Item{};for rows.Next(){var x Item;if err:=rows.Scan(&x.ItemID,&x.Folder,&x.Role,&x.IsRead,&x.IsStarred,&x.CreatedAt,&x.Message.ID,&x.Message.ThreadID,&x.Message.Subject,&x.Message.BodyText,&x.Message.BodyHTML,&x.Message.Provenance,&x.Message.State,&x.Message.SenderAddress,&x.Message.SentAt,&x.Message.CreatedAt,&x.Message.UpdatedAt);err!=nil{return nil,err};out=append(out,x)};return out,rows.Err()
}

func (r Repository) PruneActionBuckets(ctx context.Context,now time.Time)error{if r.DB==nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};_,err:=r.DB.Exec(ctx,`DELETE FROM mail_action_buckets WHERE bucket_start<$1`,now.UTC().Add(-7*24*time.Hour).Truncate(time.Hour));return err}
