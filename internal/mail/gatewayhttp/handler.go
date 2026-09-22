package gatewayhttp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

const maxGatewayJSONBytes int64 = 29 << 20

type Handler struct {
	Repo mailcore.Repository
	Inbound mailcore.InboundStore
	Secret []byte
	Enabled bool
}

type recipientRequest struct{Recipient string `json:"recipient"`}
type inboundRequest struct{EventID string `json:"event_id"`;Recipient string `json:"recipient"`;RawBase64 string `json:"raw_base64"`}
type deliveryEventRequest struct{EventID string `json:"event_id"`;DeliveryID int64 `json:"delivery_id"`;Type string `json:"type"`;RemoteQueueID string `json:"remote_queue_id"`;Code string `json:"code"`;Detail string `json:"detail"`}

func (h Handler) Routes() http.Handler {r:=chi.NewRouter();r.Post("/recipient",h.Recipient);r.Post("/inbound",h.InboundMessage);r.Post("/delivery-event",h.DeliveryEvent);return r}
func (h Handler) verify(w http.ResponseWriter,r *http.Request,max int64)([]byte,bool){
	if !h.Enabled||len(h.Secret)<32||h.Repo.DB==nil{writeError(w,http.StatusServiceUnavailable,"gateway_unavailable");return nil,false}
	r.Body=http.MaxBytesReader(w,r.Body,max);body,err:=io.ReadAll(r.Body);if err!=nil{writeError(w,http.StatusRequestEntityTooLarge,"request_too_large");return nil,false}
	nonce:=r.Header.Get("X-Poisk-Gateway-Event");ts:=r.Header.Get("X-Poisk-Gateway-Timestamp");sig:=r.Header.Get("X-Poisk-Gateway-Signature")
	hash,_,err:=mailcore.VerifyGatewayRequest(h.Secret,nonce,ts,body,sig,time.Now().UTC());if err!=nil{writeError(w,http.StatusUnauthorized,"gateway_auth_failed");return nil,false}
	if err=h.Repo.ClaimGatewayEvent(r.Context(),nonce,hash,time.Now().UTC());err!=nil{if errors.Is(err,mailcore.ErrGatewayReplay){writeError(w,http.StatusConflict,"gateway_replay");return nil,false};writeError(w,http.StatusUnauthorized,"gateway_auth_failed");return nil,false};return body,true
}
func decodeOne(body []byte,dst any)error{dec:=json.NewDecoder(bytes.NewReader(body));dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func (h Handler) Recipient(w http.ResponseWriter,r *http.Request){body,ok:=h.verify(w,r,8<<10);if !ok{return};var in recipientRequest;if err:=decodeOne(body,&in);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return};if _,err:=h.Inbound.ResolveRecipient(r.Context(),in.Recipient);err!=nil{if errors.Is(err,mailcore.ErrNotFound){writeError(w,http.StatusNotFound,"recipient_not_found");return};writeError(w,http.StatusBadRequest,"invalid_recipient");return};writeJSON(w,http.StatusOK,map[string]any{"accepted":true})}
func (h Handler) InboundMessage(w http.ResponseWriter,r *http.Request){body,ok:=h.verify(w,r,maxGatewayJSONBytes);if !ok{return};var in inboundRequest;if err:=decodeOne(body,&in);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return};raw,err:=base64.StdEncoding.DecodeString(in.RawBase64);if err!=nil||len(raw)==0||len(raw)>mailcore.MaxInboundRawBytes{writeError(w,http.StatusBadRequest,"invalid_mime");return};messageID,duplicate,err:=h.Inbound.Ingest(r.Context(),in.EventID,in.Recipient,raw,time.Now().UTC());if err!=nil{switch{case errors.Is(err,mailcore.ErrNotFound):writeError(w,http.StatusNotFound,"recipient_not_found");case errors.Is(err,mailcore.ErrDangerousMailPart):writeError(w,http.StatusUnprocessableEntity,"dangerous_mime_part");case errors.Is(err,mailcore.ErrConflict):writeError(w,http.StatusConflict,"inbound_event_conflict");case errors.Is(err,mailcore.ErrRateLimited):writeError(w,http.StatusInsufficientStorage,"mailbox_quota_exceeded");case errors.Is(err,mailcore.ErrInvalid):writeError(w,http.StatusBadRequest,"invalid_mime");default:writeError(w,http.StatusServiceUnavailable,"inbound_unavailable")};return};writeJSON(w,http.StatusOK,map[string]any{"accepted":true,"message_id":messageID,"duplicate":duplicate})}
func (h Handler) DeliveryEvent(w http.ResponseWriter,r *http.Request){body,ok:=h.verify(w,r,32<<10);if !ok{return};var in deliveryEventRequest;if err:=decodeOne(body,&in);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return};duplicate,err:=h.Repo.ApplyOutboundCallback(r.Context(),mailcore.OutboundCallback{SourceEventID:in.EventID,DeliveryID:in.DeliveryID,Type:in.Type,RemoteQueueID:in.RemoteQueueID,Code:in.Code,Detail:in.Detail},time.Now().UTC());if err!=nil{switch{case errors.Is(err,mailcore.ErrNotFound):writeError(w,http.StatusNotFound,"delivery_not_found");case errors.Is(err,mailcore.ErrGatewayMismatch):writeError(w,http.StatusConflict,"delivery_event_mismatch");case errors.Is(err,mailcore.ErrConflict):writeError(w,http.StatusConflict,"delivery_state_conflict");case errors.Is(err,mailcore.ErrInvalid):writeError(w,http.StatusBadRequest,"invalid_delivery_event");default:writeError(w,http.StatusServiceUnavailable,"delivery_event_unavailable")};return};writeJSON(w,http.StatusOK,map[string]any{"accepted":true,"duplicate":duplicate})}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
