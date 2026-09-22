package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/admin"
)

type contextKey string
const sessionKey contextKey="admin_session"
const sessionCookie="poisk_admin_session"

type OpsReader interface{Snapshot(context.Context)(admin.OpsSnapshot,error)}
type OwnerReader interface{Snapshot(context.Context)(admin.OwnerSnapshot,error)}
type DiagnosticsReader interface{Snapshot(context.Context)(admin.DiagnosticsSnapshot,error)}
type Handler struct{Service *admin.Service;Ops OpsReader;Owner OwnerReader;Diagnostics DiagnosticsReader;SecureCookies bool}

type loginRequest struct{Email string `json:"email"`;Password string `json:"password"`;SecondFactor string `json:"second_factor"`}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter()
	r.Post("/login",h.Login)
	r.Group(func(protected chi.Router){
		protected.Use(h.requireSession)
		protected.Get("/me",h.Me)
		protected.Get("/csrf",h.RotateCSRF)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/status",h.Status)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/domains",h.Domains)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/diagnostics",h.DiagnosticsSnapshot)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/query-gaps",h.QueryGaps)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/users",h.ConsumerUsers)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER","SUPPORT")).Get("/webmaster/sites",h.WebmasterSites)
		protected.With(h.RequireRoles("OPERATOR","ANALYST","VIEWER")).Get("/billing/invoices",h.BillingInvoices)
		protected.With(h.RequireRoles("OPERATOR")).Get("/users/sessions",h.ConsumerSessions)
		protected.With(h.RequireRoles("OPERATOR")).Get("/users/security-events",h.ConsumerSecurityEvents)
		protected.With(h.RequireRoles("SUPERADMIN")).Get("/owner",h.OwnerDashboard)
		protected.With(h.requireCSRF).Post("/logout",h.Logout)
		protected.With(h.RequireRoles("OPERATOR"),h.requireCSRF).Post("/domains/preview",h.DomainPreview)
		protected.With(h.RequireRoles("OPERATOR"),h.requireCSRF).Post("/domains/apply",h.DomainApply)
		protected.With(h.RequireRoles("OPERATOR"),h.requireCSRF).Post("/query-gaps/preview",h.QueryGapPreview)
		protected.With(h.RequireRoles("OPERATOR"),h.requireCSRF).Post("/query-gaps/apply",h.QueryGapApply)
	})
	return r
}

func (h Handler) Login(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	var in loginRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	result,err:=h.Service.Login(r.Context(),in.Email,in.Password,in.SecondFactor,r.UserAgent());if err!=nil{switch{case errors.Is(err,admin.ErrAdminLocked):writeError(w,http.StatusTooManyRequests,"admin_locked");case errors.Is(err,admin.ErrAdminDisabled):writeError(w,http.StatusForbidden,"admin_disabled");default:writeError(w,http.StatusUnauthorized,"invalid_credentials")};return}
	h.setSessionCookie(w,result.SessionToken,result.Session.ExpiresAt)
	writeJSON(w,http.StatusOK,map[string]any{"csrf_token":result.CSRFToken,"admin":map[string]any{"id":result.Session.AdminID,"email":result.Session.Email,"role":result.Session.Role},"expires_at":result.Session.ExpiresAt})
}

func (h Handler) Me(w http.ResponseWriter,r *http.Request){session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return};writeJSON(w,http.StatusOK,map[string]any{"admin_id":session.AdminID,"email":session.Email,"role":session.Role,"expires_at":session.ExpiresAt})}
func (h Handler) Status(w http.ResponseWriter,r *http.Request){if h.Ops==nil{writeError(w,http.StatusServiceUnavailable,"ops_unavailable");return};snapshot,err:=h.Ops.Snapshot(r.Context());if err!=nil{writeError(w,http.StatusServiceUnavailable,"ops_unavailable");return};writeJSON(w,http.StatusOK,snapshot)}
func (h Handler) OwnerDashboard(w http.ResponseWriter,r *http.Request){if h.Owner==nil{writeError(w,http.StatusServiceUnavailable,"owner_dashboard_unavailable");return};snapshot,err:=h.Owner.Snapshot(r.Context());if err!=nil{writeError(w,http.StatusServiceUnavailable,"owner_dashboard_unavailable");return};writeJSON(w,http.StatusOK,snapshot)}
func (h Handler) DiagnosticsSnapshot(w http.ResponseWriter,r *http.Request){if h.Diagnostics==nil{writeError(w,http.StatusServiceUnavailable,"diagnostics_unavailable");return};snapshot,err:=h.Diagnostics.Snapshot(r.Context());if err!=nil{writeError(w,http.StatusServiceUnavailable,"diagnostics_unavailable");return};writeJSON(w,http.StatusOK,snapshot)}
func (h Handler) Logout(w http.ResponseWriter,r *http.Request){session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return};if err:=h.Service.Logout(r.Context(),session);err!=nil{writeError(w,http.StatusServiceUnavailable,"logout_failed");return};h.clearSessionCookie(w);writeJSON(w,http.StatusOK,map[string]bool{"ok":true})}

func (h Handler) requireSession(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return};cookie,err:=r.Cookie(sessionCookie);if err!=nil||strings.TrimSpace(cookie.Value)==""{writeError(w,http.StatusUnauthorized,"unauthorized");return};session,err:=h.Service.Authenticate(r.Context(),cookie.Value);if err!=nil{h.clearSessionCookie(w);writeError(w,http.StatusUnauthorized,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),sessionKey,session)))})}
func (h Handler) requireCSRF(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return};if err:=h.Service.VerifyCSRF(session,r.Header.Get("X-CSRF-Token"));err!=nil{writeError(w,http.StatusForbidden,"csrf_failed");return};next.ServeHTTP(w,r)})}
func (h Handler) RequireRoles(roles ...string)func(http.Handler)http.Handler{return func(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return};if err:=h.Service.RequireRole(session,roles...);err!=nil{writeError(w,http.StatusForbidden,"forbidden");return};next.ServeHTTP(w,r)})}}

func SessionFromContext(ctx context.Context)(admin.Session,bool){value,ok:=ctx.Value(sessionKey).(admin.Session);return value,ok}
func (h Handler) setSessionCookie(w http.ResponseWriter,token string,expires time.Time){http.SetCookie(w,&http.Cookie{Name:sessionCookie,Value:token,Path:"/api/admin",Expires:expires,MaxAge:int(time.Until(expires).Seconds()),HttpOnly:true,Secure:h.SecureCookies,SameSite:http.SameSiteStrictMode})}
func (h Handler) clearSessionCookie(w http.ResponseWriter){http.SetCookie(w,&http.Cookie{Name:sessionCookie,Value:"",Path:"/api/admin",MaxAge:-1,Expires:time.Unix(0,0),HttpOnly:true,Secure:h.SecureCookies,SameSite:http.SameSiteStrictMode})}
func decodeOne(w http.ResponseWriter,r *http.Request,dst any,max int64)error{r.Body=http.MaxBytesReader(w,r.Body,max);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing JSON")};return err}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
