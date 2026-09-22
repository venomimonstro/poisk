package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/identity"
	identityhttp "github.com/venomimonstro/poisk/internal/identity/httpapi"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

type Handler struct{Repo mailcore.Repository;Store mailcore.AttachmentStore;Identity *identity.Service}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Use(h.requireAuth)
	r.Get("/me",h.Me);r.Get("/folders/{kind}",h.ListFolder);r.Get("/items/{itemID}",h.GetItem);r.Get("/search",h.Search);r.Get("/attachments/{attachmentID}",h.DownloadAttachment)
	r.With(h.requireCSRF).Post("/drafts",h.CreateDraft);r.With(h.requireCSRF).Put("/drafts/{messageID}",h.UpdateDraft);r.With(h.requireCSRF).Post("/drafts/{messageID}/send",h.SendDraft);r.With(h.requireCSRF).Post("/drafts/{messageID}/attachments",h.UploadAttachment)
	r.With(h.requireCSRF).Post("/items/{itemID}/read",h.SetRead);r.With(h.requireCSRF).Post("/items/{itemID}/star",h.SetStar);r.With(h.requireCSRF).Post("/items/{itemID}/trash",h.Trash);r.With(h.requireCSRF).Post("/items/{itemID}/restore",h.Restore);r.With(h.requireCSRF).Post("/items/{itemID}/spam",h.Spam)
	return r
}

func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Identity==nil{writeError(w,503,"mail_unavailable");return};cookie,err:=r.Cookie(identityhttp.SessionCookieName);if err!=nil||cookie.Value==""{writeError(w,401,"unauthorized");return};u,s,err:=h.Identity.Authenticate(r.Context(),cookie.Value);if err!=nil{writeError(w,401,"unauthorized");return};next.ServeHTTP(w,r.WithContext(withAuth(r.Context(),authed{User:u,Session:s})))})}
func (h Handler) requireCSRF(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok||!identity.VerifyCSRF(a.Session,r.Header.Get("X-CSRF-Token")){writeError(w,403,"csrf_failed");return};next.ServeHTTP(w,r)})}
func parseID(raw string)(int64,error){v,err:=strconv.ParseInt(raw,10,64);if err!=nil||v<=0{return 0,mailcore.ErrInvalid};return v,nil}
func decode(w http.ResponseWriter,r *http.Request,dst any,max int64)error{r.Body=http.MaxBytesReader(w,r.Body,max);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
func writeRepoError(w http.ResponseWriter,err error){switch{case errors.Is(err,mailcore.ErrInvalid):writeError(w,400,"invalid_request");case errors.Is(err,mailcore.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,mailcore.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,mailcore.ErrConflict):writeError(w,409,"conflict");case errors.Is(err,mailcore.ErrRateLimited):writeError(w,429,"rate_limited");default:writeError(w,503,"mail_unavailable")}}

func (h Handler) Me(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);box,err:=h.Repo.EnsureMailbox(r.Context(),a.User.ID);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,box)}
func (h Handler) ListFolder(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);limit:=50;if raw:=r.URL.Query().Get("limit");raw!=""{v,e:=strconv.Atoi(raw);if e!=nil||v<1||v>100{writeError(w,400,"invalid_limit");return};limit=v};var before int64;if raw:=r.URL.Query().Get("before_id");raw!=""{v,e:=strconv.ParseInt(raw,10,64);if e!=nil||v<=0{writeError(w,400,"invalid_cursor");return};before=v};items,err:=h.Repo.ListFolder(r.Context(),a.User.ID,chi.URLParam(r,"kind"),limit,before);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"items":items})}
func (h Handler) GetItem(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};item,err:=h.Repo.GetItem(r.Context(),a.User.ID,id);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,item)}
func (h Handler) Search(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);limit:=50;if raw:=r.URL.Query().Get("limit");raw!=""{v,e:=strconv.Atoi(raw);if e!=nil||v<1||v>100{writeError(w,400,"invalid_limit");return};limit=v};items,err:=h.Repo.Search(r.Context(),a.User.ID,r.URL.Query().Get("q"),limit);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"items":items})}

type draftRequest struct{Subject string `json:"subject"`;BodyText string `json:"body_text"`;Recipients []mailcore.Recipient `json:"recipients"`;Provenance string `json:"provenance"`;ParentMessageID *int64 `json:"parent_message_id"`}
func toDraft(in draftRequest)mailcore.DraftInput{return mailcore.DraftInput{Subject:in.Subject,BodyText:in.BodyText,Recipients:in.Recipients,Provenance:in.Provenance,ParentMessageID:in.ParentMessageID}}
func (h Handler) CreateDraft(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);var in draftRequest;if err:=decode(w,r,&in,512<<10);err!=nil{writeError(w,400,"invalid_json");return};out,err:=h.Repo.CreateDraft(r.Context(),a.User.ID,toDraft(in));if err!=nil{writeRepoError(w,err);return};writeJSON(w,201,out)}
func (h Handler) UpdateDraft(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"messageID"));if err!=nil{writeError(w,400,"invalid_message_id");return};var in draftRequest;if err=decode(w,r,&in,512<<10);err!=nil{writeError(w,400,"invalid_json");return};out,err:=h.Repo.UpdateDraft(r.Context(),a.User.ID,id,toDraft(in));if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,out)}
func (h Handler) SendDraft(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"messageID"));if err!=nil{writeError(w,400,"invalid_message_id");return};out,err:=h.Repo.SendDraft(r.Context(),a.User.ID,id,time.Now().UTC());if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,out)}

type boolRequest struct{Value bool `json:"value"`}
func (h Handler) SetRead(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};var in boolRequest;if err=decode(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};if err=h.Repo.SetRead(r.Context(),a.User.ID,id,in.Value);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) SetStar(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};var in boolRequest;if err=decode(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};if err=h.Repo.SetStarred(r.Context(),a.User.ID,id,in.Value);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) Trash(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};if err=h.Repo.MoveToTrash(r.Context(),a.User.ID,id);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) Restore(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};if err=h.Repo.Restore(r.Context(),a.User.ID,id);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) Spam(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"itemID"));if err!=nil{writeError(w,400,"invalid_item_id");return};var in boolRequest;if err=decode(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};if err=h.Repo.MoveSpam(r.Context(),a.User.ID,id,in.Value);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}

func (h Handler) UploadAttachment(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);messageID,err:=parseID(chi.URLParam(r,"messageID"));if err!=nil{writeError(w,400,"invalid_message_id");return};r.Body=http.MaxBytesReader(w,r.Body,(25<<20)+(1<<20));reader,err:=r.MultipartReader();if err!=nil{writeError(w,400,"invalid_multipart");return};part,err:=reader.NextPart();if err!=nil{writeError(w,400,"file_required");return};defer part.Close();if part.FormName()!="file"||part.FileName()==""{writeError(w,400,"file_required");return};out,err:=h.Store.Upload(r.Context(),a.User.ID,messageID,part.FileName(),part.Header.Get("Content-Type"),part,time.Now().UTC());if err!=nil{writeRepoError(w,err);return};if next,e:=reader.NextPart();e==nil&&next!=nil{_ = next.Close();writeError(w,400,"single_file_only");return};writeJSON(w,201,out)}
func (h Handler) DownloadAttachment(w http.ResponseWriter,r *http.Request){a,_:=authFrom(r);id,err:=parseID(chi.URLParam(r,"attachmentID"));if err!=nil{writeError(w,400,"invalid_attachment_id");return};file,err:=h.Store.ResolveDownload(r.Context(),a.User.ID,id);if err!=nil{writeRepoError(w,err);return};f,err:=os.Open(file.Path);if err!=nil{writeError(w,404,"not_found");return};defer f.Close();w.Header().Set("Content-Type","application/octet-stream");w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Content-Security-Policy","sandbox");w.Header().Set("Cache-Control","private, no-store");w.Header().Set("Content-Disposition",mime.FormatMediaType("attachment",map[string]string{"filename":file.Attachment.Filename}));w.Header().Set("Content-Length",strconv.FormatInt(file.Attachment.ByteSize,10));w.WriteHeader(200);_,_=io.Copy(w,f)}

var _=fmt.Sprintf
var _=strings.TrimSpace
