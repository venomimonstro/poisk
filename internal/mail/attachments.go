package mail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxAttachmentBytes int64 = 25 << 20

type AttachmentStore struct {
	Repo Repository
	Root string
}

type Attachment struct {
	ID int64 `json:"attachment_id"`
	MessageID int64 `json:"message_id"`
	Filename string `json:"filename"`
	ByteSize int64 `json:"byte_size"`
	ContentType string `json:"content_type"`
	SHA256 string `json:"sha256"`
}

type AttachmentFile struct {
	Attachment Attachment
	Path string
}

func randomUUID() (string,error){
	var b [16]byte;if _,err:=rand.Read(b[:]);err!=nil{return "",err};b[6]=(b[6]&0x0f)|0x40;b[8]=(b[8]&0x3f)|0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",b[0:4],b[4:6],b[6:8],b[8:10],b[10:16]),nil
}

func safeFilename(raw string)(string,error){
	raw=strings.TrimSpace(raw);if raw==""||len([]rune(raw))>255||raw=="."||raw==".."||strings.ContainsAny(raw,"/\\")||strings.ContainsRune(raw,'\x00'){return "",ErrInvalid};return raw,nil
}

func normalizeContentType(raw string) string {raw=strings.TrimSpace(raw);if raw==""||len(raw)>255||strings.ContainsAny(raw,"\r\n\x00"){return "application/octet-stream"};return raw}

func (s AttachmentStore) Prepare() error {
	if s.Repo.DB==nil||strings.TrimSpace(s.Root)==""{return ErrInvalid}
	return os.MkdirAll(s.Root,0700)
}

func (s AttachmentStore) Upload(ctx context.Context,userID,messageID int64,filename,contentType string,src io.Reader,now time.Time)(Attachment,error){
	if userID<=0||messageID<=0||src==nil{return Attachment{},ErrInvalid};var err error;if filename,err=safeFilename(filename);err!=nil{return Attachment{},err};contentType=normalizeContentType(contentType);if now.IsZero(){now=time.Now().UTC()};if err=s.Prepare();err!=nil{return Attachment{},err}
	box,err:=s.Repo.EnsureMailbox(ctx,userID);if err!=nil{return Attachment{},err}
	storageKey,err:=randomUUID();if err!=nil{return Attachment{},err};blobID,err:=randomUUID();if err!=nil{return Attachment{},err}
	tmp,err:=os.CreateTemp(s.Root,"upload-*.tmp");if err!=nil{return Attachment{},err};tmpPath:=tmp.Name();committed:=false;defer func(){_ = tmp.Close();if !committed{_ = os.Remove(tmpPath)}}();if err=os.Chmod(tmpPath,0600);err!=nil{return Attachment{},err}
	h:=sha256.New();n,err:=io.Copy(io.MultiWriter(tmp,h),io.LimitReader(src,maxAttachmentBytes+1));if err!=nil{return Attachment{},err};if n<=0||n>maxAttachmentBytes{return Attachment{},ErrInvalid};if err=tmp.Sync();err!=nil{return Attachment{},err};if err=tmp.Close();err!=nil{return Attachment{},err}
	finalPath:=filepath.Join(s.Root,storageKey+".blob");if err=os.Rename(tmpPath,finalPath);err!=nil{return Attachment{},err};tmpPath=finalPath
	digest:=hex.EncodeToString(h.Sum(nil));tx,err:=s.Repo.DB.Begin(ctx);if err!=nil{return Attachment{},err};defer func(){_=tx.Rollback(ctx)}()
	var quota,used int64;var active bool
	err=tx.QueryRow(ctx,`SELECT storage_quota_bytes,storage_used_bytes,status='ACTIVE' FROM mailboxes WHERE mailbox_id=$1 FOR UPDATE`,box.ID).Scan(&quota,&used,&active);if err!=nil{return Attachment{},err};if !active{return Attachment{},ErrForbidden};if used+n>quota{return Attachment{},ErrRateLimited}
	var owned bool;var count int
	if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_messages m WHERE m.message_id=$1 AND m.sender_mailbox_id=$2 AND m.state='DRAFT'),(SELECT count(*) FROM mail_attachments WHERE message_id=$1)`,messageID,box.ID).Scan(&owned,&count);err!=nil{return Attachment{},err};if !owned{return Attachment{},ErrForbidden};if count>=20{return Attachment{},ErrRateLimited}
	if err=consumeBucket(ctx,tx,box.ID,"UPLOAD_BYTES",n,100<<20,now);err!=nil{return Attachment{},err}
	if _,err=tx.Exec(ctx,`INSERT INTO mail_attachment_blobs(blob_id,storage_key,sha256,byte_size,content_type) VALUES($1::uuid,$2::uuid,$3,$4,$5)`,blobID,storageKey,digest,n,contentType);err!=nil{return Attachment{},err}
	var out Attachment;err=tx.QueryRow(ctx,`INSERT INTO mail_attachments(message_id,blob_id,original_filename,ordinal) SELECT $1,$2::uuid,$3,COALESCE(max(ordinal),0)+1 FROM mail_attachments WHERE message_id=$1 RETURNING attachment_id,message_id,original_filename`,messageID,blobID,filename).Scan(&out.ID,&out.MessageID,&out.Filename);if err!=nil{return Attachment{},err};if _,err=tx.Exec(ctx,`UPDATE mailboxes SET storage_used_bytes=storage_used_bytes+$2,updated_at=now() WHERE mailbox_id=$1`,box.ID,n);err!=nil{return Attachment{},err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) VALUES($1,$2,'ATTACH',jsonb_build_object('attachment_id',$3,'bytes',$4))`,box.ID,messageID,out.ID,n);err!=nil{return Attachment{},err};if err=tx.Commit(ctx);err!=nil{return Attachment{},err}
	committed=true;out.ByteSize=n;out.ContentType=contentType;out.SHA256=digest;return out,nil
}

func (s AttachmentStore) List(ctx context.Context,userID,messageID int64)([]Attachment,error){
	if s.Repo.DB==nil||userID<=0||messageID<=0{return nil,ErrInvalid};box,err:=s.Repo.EnsureMailbox(ctx,userID);if err!=nil{return nil,err};var visible bool;if err=s.Repo.DB.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM mail_items WHERE mailbox_id=$1 AND message_id=$2)`,box.ID,messageID).Scan(&visible);err!=nil{return nil,err};if !visible{return nil,ErrNotFound}
	rows,err:=s.Repo.DB.Query(ctx,`SELECT a.attachment_id,a.message_id,a.original_filename,b.byte_size,b.content_type,b.sha256 FROM mail_attachments a JOIN mail_attachment_blobs b ON b.blob_id=a.blob_id WHERE a.message_id=$1 ORDER BY a.ordinal`,messageID);if err!=nil{return nil,err};defer rows.Close();out:=[]Attachment{};for rows.Next(){var a Attachment;if err=rows.Scan(&a.ID,&a.MessageID,&a.Filename,&a.ByteSize,&a.ContentType,&a.SHA256);err!=nil{return nil,err};out=append(out,a)};return out,rows.Err()
}

func (s AttachmentStore) Detach(ctx context.Context,userID,messageID,attachmentID int64)error{
	if s.Repo.DB==nil||userID<=0||messageID<=0||attachmentID<=0{return ErrInvalid};box,err:=s.Repo.EnsureMailbox(ctx,userID);if err!=nil{return err};tx,err:=s.Repo.DB.Begin(ctx);if err!=nil{return err};defer func(){_=tx.Rollback(ctx)}()
	var blobID,storageKey string;var size int64;err=tx.QueryRow(ctx,`SELECT b.blob_id::text,b.storage_key::text,b.byte_size FROM mail_attachments a JOIN mail_attachment_blobs b ON b.blob_id=a.blob_id JOIN mail_messages m ON m.message_id=a.message_id WHERE a.attachment_id=$1 AND a.message_id=$2 AND m.sender_mailbox_id=$3 AND m.state='DRAFT' FOR UPDATE OF a`,attachmentID,messageID,box.ID).Scan(&blobID,&storageKey,&size);if errors.Is(err,pgx.ErrNoRows){return ErrNotFound};if err!=nil{return err}
	if _,err=tx.Exec(ctx,`DELETE FROM mail_attachments WHERE attachment_id=$1`,attachmentID);err!=nil{return err};if _,err=tx.Exec(ctx,`DELETE FROM mail_attachment_blobs WHERE blob_id=$1::uuid`,blobID);err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_blob_gc(storage_key,byte_size) VALUES($1::uuid,$2) ON CONFLICT(storage_key) DO NOTHING`,storageKey,size);err!=nil{return err};if _,err=tx.Exec(ctx,`UPDATE mailboxes SET storage_used_bytes=GREATEST(0,storage_used_bytes-$2),updated_at=now() WHERE mailbox_id=$1`,box.ID,size);err!=nil{return err};if _,err=tx.Exec(ctx,`INSERT INTO mail_events(mailbox_id,message_id,event_type,details) VALUES($1,$2,'DETACH',jsonb_build_object('attachment_id',$3,'bytes',$4))`,box.ID,messageID,attachmentID,size);err!=nil{return err};return tx.Commit(ctx)
}

func (s AttachmentStore) ResolveDownload(ctx context.Context,userID,attachmentID int64)(AttachmentFile,error){
	if s.Repo.DB==nil||userID<=0||attachmentID<=0||strings.TrimSpace(s.Root)==""{return AttachmentFile{},ErrInvalid};box,err:=s.Repo.EnsureMailbox(ctx,userID);if err!=nil{return AttachmentFile{},err};var out Attachment;var storageKey string
	err=s.Repo.DB.QueryRow(ctx,`SELECT a.attachment_id,a.message_id,a.original_filename,b.byte_size,b.content_type,b.sha256,b.storage_key::text
FROM mail_attachments a JOIN mail_attachment_blobs b ON b.blob_id=a.blob_id
WHERE a.attachment_id=$1 AND EXISTS(SELECT 1 FROM mail_items i WHERE i.mailbox_id=$2 AND i.message_id=a.message_id)`,attachmentID,box.ID).Scan(&out.ID,&out.MessageID,&out.Filename,&out.ByteSize,&out.ContentType,&out.SHA256,&storageKey)
	if errors.Is(err,pgx.ErrNoRows){return AttachmentFile{},ErrNotFound};if err!=nil{return AttachmentFile{},err}
	if _,err=safeStorageKey(storageKey);err!=nil{return AttachmentFile{},err};path:=filepath.Join(s.Root,storageKey+".blob");info,err:=os.Stat(path);if errors.Is(err,os.ErrNotExist){return AttachmentFile{},ErrNotFound};if err!=nil{return AttachmentFile{},err};if !info.Mode().IsRegular()||info.Size()!=out.ByteSize{return AttachmentFile{},ErrNotFound};return AttachmentFile{Attachment:out,Path:path},nil
}

func safeStorageKey(v string)(string,error){if len(v)!=36{return "",ErrInvalid};for i,r:=range v{if i==8||i==13||i==18||i==23{if r!='-'{return "",ErrInvalid};continue};if !((r>='0'&&r<='9')||(r>='a'&&r<='f')){return "",ErrInvalid}};return v,nil}
