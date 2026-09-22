package mail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const MaxOutboundTransportAttachmentTotal int64 = 50 << 20

type MTAAttachment struct {
	Filename string `json:"filename"`
	ContentType string `json:"content_type"`
	SHA256 string `json:"sha256"`
	DataBase64 string `json:"data_base64"`
}

type MTAEnvelope struct {
	DeliveryID int64 `json:"delivery_id"`
	IdempotencyKey string `json:"idempotency_key"`
	From string `json:"from"`
	Recipient string `json:"recipient"`
	RecipientType string `json:"recipient_type"`
	Subject string `json:"subject"`
	BodyText string `json:"body_text"`
	Attachments []MTAAttachment `json:"attachments,omitempty"`
}

type MTAError struct {Permanent bool;Code string;Detail string}
func (e *MTAError) Error()string{if e==nil{return ""};if e.Code!=""{return e.Code+": "+e.Detail};return e.Detail}

type MTAClient struct {
	BaseURL string
	Secret []byte
	HTTP *http.Client
}

type mtaSubmitResponse struct{RemoteQueueID string `json:"remote_queue_id"`}

func (r Repository) LoadOutboundEnvelope(ctx context.Context,d OutboundDelivery,blobRoot string)(MTAEnvelope,error){
	if r.DB==nil||d.ID<=0||d.MessageID<=0||d.SenderMailboxID<=0||strings.TrimSpace(blobRoot)==""{return MTAEnvelope{},ErrInvalid}
	alias,err:=r.PrimaryExternalAliasForMailbox(ctx,d.SenderMailboxID);if err!=nil{return MTAEnvelope{},err}
	var state,subject,body string
	err=r.DB.QueryRow(ctx,`SELECT state,subject,body_text FROM mail_messages WHERE message_id=$1 AND sender_mailbox_id=$2`,d.MessageID,d.SenderMailboxID).Scan(&state,&subject,&body);if errors.Is(err,pgx.ErrNoRows){return MTAEnvelope{},ErrNotFound};if err!=nil{return MTAEnvelope{},err};if state!="SENT"{return MTAEnvelope{},ErrConflict}
	var address,kind string
	err=r.DB.QueryRow(ctx,`SELECT address,recipient_type FROM mail_external_recipients WHERE external_recipient_id=$1 AND message_id=$2`,d.ExternalRecipientID,d.MessageID).Scan(&address,&kind);if errors.Is(err,pgx.ErrNoRows){return MTAEnvelope{},ErrNotFound};if err!=nil{return MTAEnvelope{},err}
	envelope:=MTAEnvelope{DeliveryID:d.ID,IdempotencyKey:d.IdempotencyKey,From:alias.Address,Recipient:address,RecipientType:kind,Subject:subject,BodyText:body}
	rows,err:=r.DB.Query(ctx,`SELECT a.original_filename,b.content_type,b.sha256,b.byte_size,b.storage_key::text FROM mail_attachments a JOIN mail_attachment_blobs b ON b.blob_id=a.blob_id WHERE a.message_id=$1 ORDER BY a.ordinal`,d.MessageID);if err!=nil{return MTAEnvelope{},err};defer rows.Close()
	var total int64
	for rows.Next(){var name,contentType,digest,storageKey string;var size int64;if err=rows.Scan(&name,&contentType,&digest,&size,&storageKey);err!=nil{return MTAEnvelope{},err};if size<=0||size>MaxInboundAttachmentBytes{return MTAEnvelope{},ErrInvalid};total+=size;if total>MaxOutboundTransportAttachmentTotal{return MTAEnvelope{},ErrRateLimited};if _,err=safeStorageKey(storageKey);err!=nil{return MTAEnvelope{},err};path:=filepath.Join(blobRoot,storageKey+".blob");data,err:=os.ReadFile(path);if err!=nil{return MTAEnvelope{},err};if int64(len(data))!=size{return MTAEnvelope{},ErrConflict};sum:=sha256.Sum256(data);if hex.EncodeToString(sum[:])!=digest{return MTAEnvelope{},ErrConflict};envelope.Attachments=append(envelope.Attachments,MTAAttachment{Filename:name,ContentType:contentType,SHA256:digest,DataBase64:base64.StdEncoding.EncodeToString(data)})};if err=rows.Err();err!=nil{return MTAEnvelope{},err};return envelope,nil
}

func (c MTAClient) Submit(ctx context.Context,envelope MTAEnvelope)(string,error){
	if len(c.Secret)<32{return "",&MTAError{Permanent:true,Code:"MTA_CONFIG",Detail:"gateway secret is not configured"}}
	base,err:=url.Parse(strings.TrimSpace(c.BaseURL));if err!=nil||base.Host==""||(base.Scheme!="http"&&base.Scheme!="https"){return "",&MTAError{Permanent:true,Code:"MTA_CONFIG",Detail:"invalid MTA base URL"}}
	base.Path=strings.TrimRight(base.Path,"/")+"/v1/outbound";base.RawQuery="";base.Fragment=""
	body,err:=json.Marshal(envelope);if err!=nil{return "",err};nonceID,err:=randomUUID();if err!=nil{return "",err};eventID:="submit-"+nonceID;now:=time.Now().UTC();signature,err:=SignGatewayRequest(c.Secret,eventID,now,body);if err!=nil{return "",err}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,base.String(),bytes.NewReader(body));if err!=nil{return "",err};req.Header.Set("Content-Type","application/json");req.Header.Set("Accept","application/json");req.Header.Set("X-Poisk-Gateway-Event",eventID);req.Header.Set("X-Poisk-Gateway-Timestamp",fmt.Sprintf("%d",now.Unix()));req.Header.Set("X-Poisk-Gateway-Signature",signature)
	client:=c.HTTP;if client==nil{client=&http.Client{Timeout:15*time.Second}}
	resp,err:=client.Do(req);if err!=nil{return "",&MTAError{Permanent:false,Code:"MTA_UNAVAILABLE",Detail:err.Error()}};defer resp.Body.Close();payload,readErr:=io.ReadAll(io.LimitReader(resp.Body,64<<10));if readErr!=nil{return "",&MTAError{Permanent:false,Code:"MTA_RESPONSE",Detail:readErr.Error()}}
	if resp.StatusCode<200||resp.StatusCode>=300{permanent:=resp.StatusCode>=400&&resp.StatusCode<500&&resp.StatusCode!=http.StatusTooManyRequests;return "",&MTAError{Permanent:permanent,Code:fmt.Sprintf("MTA_HTTP_%d",resp.StatusCode),Detail:strings.TrimSpace(string(payload))}}
	var out mtaSubmitResponse;if err=json.Unmarshal(payload,&out);err!=nil||strings.TrimSpace(out.RemoteQueueID)==""||len(out.RemoteQueueID)>255{return "",&MTAError{Permanent:false,Code:"MTA_RESPONSE",Detail:"missing remote_queue_id"}};return strings.TrimSpace(out.RemoteQueueID),nil
}
