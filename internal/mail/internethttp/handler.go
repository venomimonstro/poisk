package internethttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/identity"
	identityhttp "github.com/venomimonstro/poisk/internal/identity/httpapi"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

type contextKey string
const authKey contextKey="internet_mail_auth"
type auth struct{User identity.User;Session identity.Session}

type Handler struct{Repo mailcore.Repository;Identity *identity.Service;Enabled bool;Domain string}

type addressRequest struct{LocalPart string `json:"local_part"`}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Use(h.requireAuth)
	r.Get("/address",h.Address)
	r.With(h.requireCSRF).Put("/address",h.ClaimAddress)
	r.Get("/messages/{messageID}/deliveries",h.Deliveries)
	return r
}

func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Identity==nil{writeError(w,503,"mail_unavailable");return};cookie,err:=r.Cookie(identityhttp.SessionCookieName);if err!=nil||cookie.Value==""{writeError(w,401,"unauthorized");return};u,s,err:=h.Identity.Authenticate(r.Context(),cookie.Value);if err!=nil{writeError(w,401,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),authKey,auth{User:u,Session:s})))})}
func (h Handler) requireCSRF(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){a,ok:=r.Context().Value(authKey).(auth);if !ok||!identity.VerifyCSRF(a.Session,r.Header.Get("X-CSRF-Token")){writeError(w,403,"csrf_failed");return};next.ServeHTTP(w,r)})}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,8<<10);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
func writeRepoError(w http.ResponseWriter,err error){switch{case errors.Is(err,mailcore.ErrInvalid):writeError(w,400,"invalid_request");case errors.Is(err,mailcore.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,mailcore.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,mailcore.ErrConflict):writeError(w,409,"conflict");default:writeError(w,503,"mail_unavailable")}}

func (h Handler) Address(w http.ResponseWriter,r *http.Request){
	if !h.Enabled||strings.TrimSpace(h.Domain)==""{writeJSON(w,200,map[string]any{"internet_enabled":false});return};a:=r.Context().Value(authKey).(auth);alias,err:=h.Repo.UserPrimaryExternalAlias(r.Context(),a.User.ID);if errors.Is(err,mailcore.ErrNotFound){writeJSON(w,200,map[string]any{"internet_enabled":true,"domain":h.Domain,"address":nil});return};if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"internet_enabled":true,"domain":h.Domain,"address":alias.Address})
}
func (h Handler) ClaimAddress(w http.ResponseWriter,r *http.Request){
	if !h.Enabled||strings.TrimSpace(h.Domain)==""{writeError(w,503,"internet_mail_disabled");return};a:=r.Context().Value(authKey).(auth);var in addressRequest;if err:=decode(w,r,&in);err!=nil{writeError(w,400,"invalid_json");return};alias,err:=h.Repo.ClaimPrimaryExternalAlias(r.Context(),a.User.ID,in.LocalPart,h.Domain);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"address":alias.Address})
}
func (h Handler) Deliveries(w http.ResponseWriter,r *http.Request){
	a:=r.Context().Value(authKey).(auth);messageID,err:=strconv.ParseInt(chi.URLParam(r,"messageID"),10,64);if err!=nil||messageID<=0{writeError(w,400,"invalid_message_id");return};items,err:=h.Repo.OutboundStatuses(r.Context(),a.User.ID,messageID);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"deliveries":items})
}
