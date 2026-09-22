package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/identity"
)

const SessionCookieName = "poisk_session"

type contextKey string
const (
	userKey contextKey="consumer-user"
	sessionKey contextKey="consumer-session"
)

type Handler struct{
	Service *identity.Service
	SecureCookies bool
}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter()
	r.Post("/register",h.Register)
	r.Post("/login",h.Login)
	r.Group(func(r chi.Router){
		r.Use(h.RequireAuth)
		r.Get("/me",h.Me)
		r.Get("/csrf",h.CSRF)
		r.Get("/sessions",h.Sessions)
		r.With(h.RequireCSRF).Post("/logout",h.Logout)
		r.With(h.RequireCSRF).Post("/sessions/{sessionID}/revoke",h.RevokeSession)
	})
	return r
}

func (h Handler) available(w http.ResponseWriter)bool{if h.Service!=nil&&h.Service.Repo!=nil{return true};writeError(w,503,"account_unavailable");return false}

func (h Handler) Register(w http.ResponseWriter,r *http.Request){
	if !h.available(w){return};var in struct{Email string `json:"email"`;Password string `json:"password"`};if !decodeJSON(w,r,&in){return}
	result,err:=h.Service.Register(r.Context(),in.Email,in.Password);if err!=nil{writeIdentityError(w,err);return};h.setSessionCookie(w,result.SessionToken,result.Session.ExpiresAt);writeJSON(w,201,map[string]any{"user":publicUser(result.User),"csrf_token":result.CSRFToken,"expires_at":result.Session.ExpiresAt})
}
func (h Handler) Login(w http.ResponseWriter,r *http.Request){
	if !h.available(w){return};var in struct{Email string `json:"email"`;Password string `json:"password"`};if !decodeJSON(w,r,&in){return}
	result,err:=h.Service.Login(r.Context(),in.Email,in.Password);if err!=nil{writeIdentityError(w,err);return};h.setSessionCookie(w,result.SessionToken,result.Session.ExpiresAt);writeJSON(w,200,map[string]any{"user":publicUser(result.User),"csrf_token":result.CSRFToken,"expires_at":result.Session.ExpiresAt})
}

func (h Handler) RequireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
	if !h.available(w){return};cookie,err:=r.Cookie(SessionCookieName);if err!=nil||strings.TrimSpace(cookie.Value)==""{writeError(w,401,"unauthorized");return}
	u,s,err:=h.Service.Authenticate(r.Context(),cookie.Value);if err!=nil{writeError(w,401,"unauthorized");return}
	ctx:=context.WithValue(r.Context(),userKey,u);ctx=context.WithValue(ctx,sessionKey,s);next.ServeHTTP(w,r.WithContext(ctx))
})}
func (h Handler) RequireCSRF(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){_,s,ok:=current(r);if !ok||!identity.VerifyCSRF(s,r.Header.Get("X-CSRF-Token")){writeError(w,403,"csrf_failed");return};next.ServeHTTP(w,r)})}

func (h Handler) Me(w http.ResponseWriter,r *http.Request){u,_,ok:=current(r);if !ok{writeError(w,401,"unauthorized");return};writeJSON(w,200,map[string]any{"user":publicUser(u)})}
func (h Handler) CSRF(w http.ResponseWriter,r *http.Request){u,s,ok:=current(r);if !ok{writeError(w,401,"unauthorized");return};token,err:=h.Service.RotateCSRF(r.Context(),u,s);if err!=nil{writeIdentityError(w,err);return};writeJSON(w,200,map[string]string{"csrf_token":token})}
func (h Handler) Logout(w http.ResponseWriter,r *http.Request){u,s,ok:=current(r);if !ok{writeError(w,401,"unauthorized");return};if err:=h.Service.Logout(r.Context(),u,s);err!=nil{writeIdentityError(w,err);return};h.clearSessionCookie(w);w.WriteHeader(204)}
func (h Handler) Sessions(w http.ResponseWriter,r *http.Request){u,_,ok:=current(r);if !ok{writeError(w,401,"unauthorized");return};items,err:=h.Service.Sessions(r.Context(),u);if err!=nil{writeIdentityError(w,err);return};writeJSON(w,200,map[string]any{"sessions":items})}
func (h Handler) RevokeSession(w http.ResponseWriter,r *http.Request){u,currentSession,ok:=current(r);if !ok{writeError(w,401,"unauthorized");return};id,err:=strconv.ParseInt(chi.URLParam(r,"sessionID"),10,64);if err!=nil||id<=0{writeError(w,400,"invalid_session_id");return};if err:=h.Service.RevokeSession(r.Context(),u,id);err!=nil{writeIdentityError(w,err);return};if id==currentSession.ID{h.clearSessionCookie(w)};w.WriteHeader(204)}

func current(r *http.Request)(identity.User,identity.Session,bool){u,ok:=r.Context().Value(userKey).(identity.User);if !ok{return identity.User{},identity.Session{},false};s,ok:=r.Context().Value(sessionKey).(identity.Session);return u,s,ok}
func publicUser(u identity.User)map[string]any{return map[string]any{"user_id":u.ID,"email":u.Email,"status":u.Status,"email_verified_at":u.EmailVerifiedAt}}
func (h Handler) setSessionCookie(w http.ResponseWriter,token string,expires time.Time){http.SetCookie(w,&http.Cookie{Name:SessionCookieName,Value:token,Path:"/",HttpOnly:true,Secure:h.SecureCookies,SameSite:http.SameSiteStrictMode,Expires:expires,MaxAge:int(time.Until(expires).Seconds())})}
func (h Handler) clearSessionCookie(w http.ResponseWriter){http.SetCookie(w,&http.Cookie{Name:SessionCookieName,Value:"",Path:"/",HttpOnly:true,Secure:h.SecureCookies,SameSite:http.SameSiteStrictMode,MaxAge:-1,Expires:time.Unix(1,0)})}
func decodeJSON(w http.ResponseWriter,r *http.Request,dst any)bool{r.Body=http.MaxBytesReader(w,r.Body,64<<10);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{writeError(w,400,"invalid_json");return false};var extra any;if err:=dec.Decode(&extra);err!=io.EOF{writeError(w,400,"invalid_json");return false};return true}
func writeIdentityError(w http.ResponseWriter,err error){switch{case errors.Is(err,identity.ErrWeakPassword),errors.Is(err,identity.ErrInvalidCredential):writeError(w,400,"invalid_request");case errors.Is(err,identity.ErrConflict):writeError(w,409,"conflict");case errors.Is(err,identity.ErrUnauthorized),errors.Is(err,identity.ErrLocked):writeError(w,401,"invalid_credentials");case errors.Is(err,identity.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,context.DeadlineExceeded),errors.Is(err,context.Canceled):writeError(w,504,"timeout");default:writeError(w,500,"account_error")}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
