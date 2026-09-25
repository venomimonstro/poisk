package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	ErrSuppressed = errors.New("mail external recipient suppressed")
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

type Recipient struct { Address string `json:"address"`; Type string `json:"type"` }
type Message struct {
	ID int64 `json:"message_id"`; ThreadID int64 `json:"thread_id"`; Subject string `json:"subject"`; BodyText string `json:"body_text"`; BodyHTML string `json:"body_html,omitempty"`; Provenance string `json:"provenance"`; State string `json:"state"`; SenderAddress string `json:"sender_address"`; Recipients []Recipient `json:"recipients,omitempty"`; SentAt *time.Time `json:"sent_at,omitempty"`; CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
type Item struct { ItemID int64 `json:"mail_item_id"`; Message Message `json:"message"`; Folder string `json:"folder"`; Role string `json:"role"`; IsRead bool `json:"is_read"`; IsStarred bool `json:"is_starred"`; CreatedAt time.Time `json:"created_at"` }
type DraftInput struct { Subject string; BodyText string; Recipients []Recipient; Provenance string; ParentMessageID *int64 }

func normalizeText(raw string,min,max int)(string,error){raw=strings.TrimSpace(raw);n:=utf8.RuneCountInString(raw);if n<min||n>max||strings.ContainsRune(raw,'\x00'){return "",ErrInvalid};return raw,nil}
func sanitizeHTMLFromText(raw string) string {escaped:=html.EscapeString(raw);return strings.ReplaceAll(escaped,"\n","<br>\n")}
func randomInternalAddress()(string,error){var b [12]byte;if _,err:=rand.Read(b[:]);err!=nil{return "",err};return "m-"+hex.EncodeToString(b[:])+"@internal.poisk",nil}

func (r Repository) EnsureMailbox(ctx context.Context,userID int64)(Mailbox,error){
	if r.DB==nil||userID<=0{return Mailbox{},ErrInvalid};address,err:=randomInternalAddress();if err!=nil{return Mailbox{},err};var out Mailbox
	err=r.DB.QueryRow(ctx,`INSERT INTO mailboxes(consumer_user_id,address) SELECT user_id,$2 FROM consumer_users WHERE user_id=$1 AND status='ACTIVE' ON CONFLICT(consumer_user_id) DO UPDATE SET updated_at=mailboxes.updated_at RETURNING mailbox_id,address,status,storage_quota_bytes,storage_used_bytes`,userID,address).Scan(&out.ID,&out.Address,&out.Status,&out.StorageQuotaBytes,&out.StorageUsedBytes);if errors.Is(err,pgx.ErrNoRows){return Mailbox{},ErrForbidden};return out,err
}

func normalizeRecipients(items []Recipient)([]Recipient,error){
	if len(items)==0||len(items)>maxRecipients{return nil,ErrInvalid};seen:=make(map[string]struct{},len(items));out:=make([]Recipient,0,len(items))
	for _,item:=range items{address:=strings.TrimSpace(item.Address);kind:=strings.ToUpper(strings.TrimSpace(item.Type));if kind==""{kind="TO"};key:=strings.ToLower(address);if address==""||len(address)>320||(kind!="TO"&&kind!="CC"&&kind!="BCC"){return nil,ErrInvalid};if _,ok:=seen[key];ok{continue};seen[key]=struct{}{};out=append(out,Recipient{Address:address,Type:kind})}
	if len(out)==0{return nil,ErrInvalid};return out,nil
}

func folderID(ctx context.Context,tx pgx.Tx,mailboxID int64,kind string)(int64,error){var id int64;err:=tx.QueryRow(ctx,`SELECT folder_id FROM mail_folders WHERE mailbox_id=$1 AND kind=$2`,mailboxID,kind).Scan(&id);return id,err}

func (r Repository) mailboxForUser(ctx context.Context,tx pgx.Tx,userID int64,lock bool)(Mailbox,error){
	q:=`SELECT m.mailbox_id,m.address,m.status,m.storage_quota_bytes,m.storage_used_bytes FROM mailboxes m JOIN consumer_users u ON u.user_id=m.consumer_user_id WHERE u.user_id=$1 AND u.status='ACTIVE'`
	if lock{q+=` FOR UPDATE OF m`};var b Mailbox;err:=tx.QueryRow(ctx,q,userID).Scan(&b.ID,&b.Address,&b.Status,&b.StorageQuotaBytes,&b.StorageUsedBytes);if errors.Is(err,pgx.ErrNoRows){return Mailbox{},ErrForbidden};if err==nil&&b.Status!="ACTIVE"{return Mailbox{},ErrForbidden};return b,err
}

func validateDraft(in DraftInput)(DraftInput,error){var err error;in.Subject,err=normalizeText(in.Subject,1,maxSubjectRunes);if err!=nil{return in,err};in.BodyText,err=normalizeText(in.BodyText,1,maxBodyRunes);if err!=nil{return in,err};in.Recipients,err=normalizeRecipients(in.Recipients);if err!=nil{return in,err};in.Provenance=strings.ToUpper(strings.TrimSpace(in.Provenance));if in.Provenance==""{in.Provenance="COMPOSE"};if in.Provenance!="COMPOSE"&&in.Provenance!="REPLY"&&in.Provenance!="FORWARD"{return in,ErrInvalid};return in,nil}

func (r Repository) CreateDraft(ctx context.Context,userID int64,in DraftInput)(Message,error){
	in,err:=validateDraft(in);if err!=nil{return Message{},err};tx,err:=r.DB.Begin(ctx);if err!=nil{return Message{},err};defer func(){_=tx.Rollback(ctx)}();box,err:=r.mailboxForUser(ctx,tx,userID,true);if err!=nil{return Message{},err};draftsID,err:=folderID(ctx,tx,box.ID,"DRAFTS");if err!=nil{return Message{},err};var threadID int64;if err=tx.QueryRow(ctx,`INSERT INTO mail_threads(subject) VALUES($1) RETURNING thread_id`,in.Subject).Scan(&threadID);err!=nil{return Message{},err};var m Message;err=tx.QueryRow(ctx,`INSERT INTO mail_messages(thread_id,sender_mailbox_id,parent_message_id,provenance,subject,body_text,body_html,state) VALUES($1,$2,$3,$4,$5,$6,$7,'DRAFT') RETURNING message_id,created_at,updated_at`,threadID,box.ID,in.ParentMessageID,in.Provenance,in.Subject,in.BodyText,sanitizeHTMLFromText(in.BodyText)).Scan(&m.ID,&m.CreatedAt,&m.UpdatedAt);if err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id) VALUES($1,$2,'DRAFT',$3)`,box.ID,m.ID,draftsID);err!=nil{return Message{},err};if err=replaceMessageRecipients(ctx,tx,m.ID,in.Recipients);err!=nil{return Message{},err};if err=tx.Commit(ctx);err!=nil{return Message{},err};m.ThreadID=threadID;m.Subject=in.Subject;m.BodyText=in.BodyText;m.BodyHTML=sanitizeHTMLFromText(in.BodyText);m.Provenance=in.Provenance;m.State="DRAFT";m.SenderAddress=box.Address;m.Recipients=in.Recipients;return m,nil
}

func (r Repository) SendDraft(ctx context.Context,userID,messageID int64,now time.Time)(Message,error){
	if now.IsZero(){now=time.Now().UTC()};tx,err:=r.DB.Begin(ctx);if err!=nil{return Message{},err};defer func(){_=tx.Rollback(ctx)}();box,err:=r.mailboxForUser(ctx,tx,userID,true);if err!=nil{return Message{},err};var m Message
	err=tx.QueryRow(ctx,`SELECT message_id,thread_id,subject,body_text,body_html,provenance,created_at,updated_at FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2 AND state='DRAFT' FOR UPDATE`,messageID,box.ID).Scan(&m.ID,&m.ThreadID,&m.Subject,&m.BodyText,&m.BodyHTML,&m.Provenance,&m.CreatedAt,&m.UpdatedAt);if errors.Is(err,pgx.ErrNoRows){return Message{},ErrNotFound};if err!=nil{return Message{},err}
	internal,external,err:=lockSendRecipients(ctx,tx,messageID);if err!=nil{return Message{},err};if err=applySendRecipients(ctx,tx,messageID,box.ID,internal,external,&m);err!=nil{return Message{},err}
	sentID,err:=folderID(ctx,tx,box.ID,"SENT");if err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`DELETE FROM mail_items WHERE mailbox_id=$1 AND message_id=$2 AND item_role='DRAFT'`,box.ID,messageID);err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`INSERT INTO mail_items(mailbox_id,message_id,item_role,folder_id,is_read) VALUES($1,$2,'SENT_COPY',$3,TRUE)`,box.ID,messageID,sentID);err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`UPDATE mail_messages SET state='SENT',sent_at=$3,updated_at=$3 WHERE message_id=$1 AND sender_mailbox_id=$2`,messageID,box.ID,now);err!=nil{return Message{},err};if _,err=tx.Exec(ctx,`UPDATE mail_threads SET updated_at=$2 WHERE thread_id=$1`,m.ThreadID,now);err!=nil{return Message{},err};if err=tx.Commit(ctx);err!=nil{return Message{},err};m.State="SENT";m.SenderAddress=box.Address;m.SentAt=&now;m.UpdatedAt=now;return m,nil
}

func (r Repository) ListFolder(ctx context.Context,userID int64,kind string,limit int,beforeID int64)([]Item,error){
	kind=strings.ToUpper(strings.TrimSpace(kind));if kind!="INBOX"&&kind!="SENT"&&kind!="DRAFTS"&&kind!="TRASH"&&kind!="SPAM"{return nil,ErrInvalid};if limit<=0{limit=50};if limit>100{limit=100};tx,err:=r.DB.Begin(ctx);if err!=nil{return nil,err};defer func(){_=tx.Rollback(ctx)}();box,err:=r.mailboxForUser(ctx,tx,userID,false);if err!=nil{return nil,err};rows,err:=tx.Query(ctx,`SELECT i.mail_item_id,i.message_id,m.thread_id,m.subject,m.body_text,m.body_html,m.provenance,m.state,m.created_at,m.updated_at,m.sent_at,i.is_read,i.is_starred,i.created_at FROM mail_items i JOIN mail_messages m ON m.message_id=i.message_id JOIN mail_folders f ON f.folder_id=i.folder_id WHERE i.mailbox_id=$1 AND f.kind=$2 AND ($3::bigint=0 OR i.mail_item_id<$3) ORDER BY i.mail_item_id DESC LIMIT $4`,box.ID,kind,beforeID,limit);if err!=nil{return nil,err};defer rows.Close();out:=[]Item{};for rows.Next(){var it Item;if err=rows.Scan(&it.ItemID,&it.Message.ID,&it.Message.ThreadID,&it.Message.Subject,&it.Message.BodyText,&it.Message.BodyHTML,&it.Message.Provenance,&it.Message.State,&it.Message.CreatedAt,&it.Message.UpdatedAt,&it.Message.SentAt,&it.IsRead,&it.IsStarred,&it.CreatedAt);err!=nil{return nil,err};it.Folder=kind;it.Message.SenderAddress=box.Address;out=append(out,it)};return out,rows.Err()
}

func (r Repository) Search(ctx context.Context,userID int64,q string,limit int)([]Item,error){
	q=strings.TrimSpace(q);if len(q)<2||len(q)>200{return nil,ErrInvalid};if limit<=0{limit=50};if limit>100{limit=100};tx,err:=r.DB.Begin(ctx);if err!=nil{return nil,err};defer func(){_=tx.Rollback(ctx)}();box,err:=r.mailboxForUser(ctx,tx,userID,false);if err!=nil{return nil,err};rows,err:=tx.Query(ctx,`SELECT i.mail_item_id,i.message_id,m.thread_id,m.subject,m.body_text,m.body_html,m.provenance,m.state,m.created_at,m.updated_at,m.sent_at,i.is_read,i.is_starred,i.created_at,ts_rank_cd(m.search_vector,websearch_to_tsquery('simple',$2)) rank FROM mail_items i JOIN mail_messages m ON m.message_id=i.message_id WHERE i.mailbox_id=$1 AND i.deleted_at IS NULL AND m.search_vector @@ websearch_to_tsquery('simple',$2) ORDER BY rank DESC,i.mail_item_id DESC LIMIT $3`,box.ID,q,limit);if err!=nil{return nil,err};defer rows.Close();out:=[]Item{};for rows.Next(){var it Item;var rank float64;if err=rows.Scan(&it.ItemID,&it.Message.ID,&it.Message.ThreadID,&it.Message.Subject,&it.Message.BodyText,&it.Message.BodyHTML,&it.Message.Provenance,&it.Message.State,&it.Message.CreatedAt,&it.Message.UpdatedAt,&it.Message.SentAt,&it.IsRead,&it.IsStarred,&it.CreatedAt,&rank);err!=nil{return nil,err};out=append(out,it)};return out,rows.Err()
}
