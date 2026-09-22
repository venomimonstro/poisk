package mail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const trashRetention = 30 * 24 * time.Hour

func (r Repository) PruneTrash(ctx context.Context,now time.Time,limit int)error{
	if r.DB==nil{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};if limit<=0{limit=200};if limit>1000{limit=1000}
	tx,err:=r.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	rows,err:=tx.Query(ctx,`SELECT i.mail_item_id,i.message_id FROM mail_items i JOIN mail_folders f ON f.folder_id=i.folder_id AND f.mailbox_id=i.mailbox_id WHERE f.kind='TRASH' AND i.deleted_at IS NOT NULL AND i.deleted_at<$1 ORDER BY i.deleted_at,i.mail_item_id FOR UPDATE OF i SKIP LOCKED LIMIT $2`,now.UTC().Add(-trashRetention),limit);if err!=nil{return err}
	type doomed struct{itemID,messageID int64};items:=[]doomed{};for rows.Next(){var x doomed;if err=rows.Scan(&x.itemID,&x.messageID);err!=nil{rows.Close();return err};items=append(items,x)};if err=rows.Err();err!=nil{rows.Close();return err};rows.Close()
	messages:=map[int64]struct{}{};for _,x:=range items{if _,err=tx.Exec(ctx,`DELETE FROM mail_items WHERE mail_item_id=$1`,x.itemID);err!=nil{return err};messages[x.messageID]=struct{}{}}
	for messageID:=range messages{
		var remaining int;if err=tx.QueryRow(ctx,`SELECT count(*) FROM mail_items WHERE message_id=$1`,messageID).Scan(&remaining);err!=nil{return err};if remaining>0{continue}
		blobRows,err:=tx.Query(ctx,`SELECT b.blob_id::text,b.storage_key::text,b.byte_size,m.sender_mailbox_id FROM mail_attachments a JOIN mail_attachment_blobs b ON b.blob_id=a.blob_id JOIN mail_messages m ON m.message_id=a.message_id WHERE a.message_id=$1`,messageID);if err!=nil{return err}
		type blob struct{id,key string;size,owner int64};blobs:=[]blob{};for blobRows.Next(){var b blob;if err=blobRows.Scan(&b.id,&b.key,&b.size,&b.owner);err!=nil{blobRows.Close();return err};blobs=append(blobs,b)};if err=blobRows.Err();err!=nil{blobRows.Close();return err};blobRows.Close()
		for _,b:=range blobs{if _,err=tx.Exec(ctx,`INSERT INTO mail_blob_gc(storage_key,byte_size) VALUES($1::uuid,$2) ON CONFLICT(storage_key) DO NOTHING`,b.key,b.size);err!=nil{return err};if _,err=tx.Exec(ctx,`DELETE FROM mail_attachments WHERE message_id=$1 AND blob_id=$2::uuid`,messageID,b.id);err!=nil{return err};if _,err=tx.Exec(ctx,`DELETE FROM mail_attachment_blobs WHERE blob_id=$1::uuid`,b.id);err!=nil{return err};if _,err=tx.Exec(ctx,`UPDATE mailboxes SET storage_used_bytes=GREATEST(0,storage_used_bytes-$2),updated_at=now() WHERE mailbox_id=$1`,b.owner,b.size);err!=nil{return err}}
		var referenced bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_messages WHERE parent_message_id=$1)`,messageID).Scan(&referenced);err!=nil{return err};if !referenced{if _,err=tx.Exec(ctx,`DELETE FROM mail_recipients WHERE message_id=$1`,messageID);err!=nil{return err};if _,err=tx.Exec(ctx,`DELETE FROM mail_messages WHERE message_id=$1`,messageID);err!=nil{return err}}
	}
	if _,err=tx.Exec(ctx,`DELETE FROM mail_threads t WHERE NOT EXISTS(SELECT 1 FROM mail_messages m WHERE m.thread_id=t.thread_id)`);err!=nil{return err}
	return tx.Commit(ctx)
}

func (s AttachmentStore) RunBlobGC(ctx context.Context,now time.Time,limit int)error{
	if s.Repo.DB==nil||strings.TrimSpace(s.Root)==""{return ErrInvalid};if now.IsZero(){now=time.Now().UTC()};if limit<=0{limit=100};if limit>500{limit=500};if err:=s.Prepare();err!=nil{return err}
	rows,err:=s.Repo.DB.Query(ctx,`SELECT storage_key::text FROM mail_blob_gc WHERE next_attempt_at<=$1 ORDER BY queued_at LIMIT $2`,now,limit);if err!=nil{return err};keys:=[]string{};for rows.Next(){var key string;if err=rows.Scan(&key);err!=nil{rows.Close();return err};keys=append(keys,key)};if err=rows.Err();err!=nil{rows.Close();return err};rows.Close()
	for _,key:=range keys{if _,err=safeStorageKey(key);err!=nil{_,_=s.Repo.DB.Exec(ctx,`UPDATE mail_blob_gc SET attempts=attempts+1,last_error='invalid storage key',next_attempt_at=$2 WHERE storage_key=$1::uuid`,key,now.Add(24*time.Hour));continue};path:=filepath.Join(s.Root,key+".blob");err=os.Remove(path);if err==nil||errors.Is(err,os.ErrNotExist){if _,dbErr:=s.Repo.DB.Exec(ctx,`DELETE FROM mail_blob_gc WHERE storage_key=$1::uuid`,key);dbErr!=nil{return dbErr};continue};msg:=err.Error();if len(msg)>1000{msg=msg[:1000]};if _,dbErr:=s.Repo.DB.Exec(ctx,`UPDATE mail_blob_gc SET attempts=attempts+1,last_error=$2,next_attempt_at=$3 WHERE storage_key=$1::uuid`,key,msg,now.Add(time.Hour));dbErr!=nil{return dbErr}}
	return nil
}

func (r Repository) PruneMaintenance(ctx context.Context,now time.Time)error{if err:=r.PruneActionBuckets(ctx,now);err!=nil{return err};return r.PruneTrash(ctx,now,200)}

var _ pgx.Tx
