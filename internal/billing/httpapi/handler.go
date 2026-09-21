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
	"github.com/venomimonstro/poisk/internal/billing"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

type Authenticator interface{Authenticate(context.Context,string)(webmaster.User,error)}
type Handler struct{Auth Authenticator;Billing *billing.Repository}
type key string
const userKey key="billing-user"

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Get("/plans",h.Plans);r.Group(func(p chi.Router){p.Use(h.requireAuth);p.Post("/accounts",h.EnsureAccount);p.Post("/subscriptions",h.Subscribe);p.Get("/accounts/{accountID}/entitlements/{product}",h.Entitlement);p.Get("/accounts/{accountID}/usage",h.Usage)});return r
}
func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Auth==nil||h.Billing==nil{writeError(w,503,"billing_unavailable");return};token:=bearer(r.Header.Get("Authorization"));if token==""{writeError(w,401,"unauthorized");return};u,err:=h.Auth.Authenticate(r.Context(),token);if err!=nil{writeError(w,401,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userKey,u)))})}
func current(r *http.Request)(webmaster.User,bool){u,ok:=r.Context().Value(userKey).(webmaster.User);return u,ok}
func (h Handler) Plans(w http.ResponseWriter,r *http.Request){if h.Billing==nil{writeError(w,503,"billing_unavailable");return};plans,err:=h.Billing.Plans(r.Context(),r.URL.Query().Get("product"));if err!=nil{writeError(w,503,"billing_error");return};writeJSON(w,200,map[string]any{"plans":plans})}
func (h Handler) EnsureAccount(w http.ResponseWriter,r *http.Request){u,_:=current(r);var in struct{OwnerType string `json:"owner_type"`;OwnerID int64 `json:"owner_id"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};var out billing.Account;var err error;switch strings.ToUpper(strings.TrimSpace(in.OwnerType)){case "USER":out,err=h.Billing.EnsureUserAccount(r.Context(),u.ID);case "AGENCY":out,err=h.Billing.EnsureAgencyAccount(r.Context(),u.ID,in.OwnerID);case "PLACE":out,err=h.Billing.EnsurePlaceAccount(r.Context(),u.ID,in.OwnerID);default:err=billing.ErrInvalid};if err!=nil{writeBillingError(w,err);return};writeJSON(w,200,out)}
func (h Handler) Subscribe(w http.ResponseWriter,r *http.Request){u,_:=current(r);var in struct{AccountID int64 `json:"account_id"`;PlanCode string `json:"plan_code"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};if _,err:=h.Billing.AccountForUser(r.Context(),u.ID,in.AccountID);err!=nil{writeBillingError(w,err);return};sub,inv,err:=h.Billing.CreatePendingSubscription(r.Context(),in.AccountID,in.PlanCode,time.Now().UTC());if err!=nil{writeBillingError(w,err);return};writeJSON(w,201,map[string]any{"subscription":sub,"invoice":inv})}
func (h Handler) Entitlement(w http.ResponseWriter,r *http.Request){u,_:=current(r);id,ok:=accountID(r);if !ok{writeError(w,400,"invalid_account_id");return};if _,err:=h.Billing.AccountForUser(r.Context(),u.ID,id);err!=nil{writeBillingError(w,err);return};ent,err:=h.Billing.Entitlement(r.Context(),id,chi.URLParam(r,"product"),time.Now().UTC());if err!=nil{writeBillingError(w,err);return};writeJSON(w,200,ent)}
func (h Handler) Usage(w http.ResponseWriter,r *http.Request){u,_:=current(r);id,ok:=accountID(r);if !ok{writeError(w,400,"invalid_account_id");return};usage,err:=h.Billing.Usage(r.Context(),u.ID,id);if err!=nil{writeBillingError(w,err);return};writeJSON(w,200,map[string]any{"usage":usage})}
func accountID(r *http.Request)(int64,bool){v,err:=strconv.ParseInt(chi.URLParam(r,"accountID"),10,64);return v,err==nil&&v>0}
func bearer(v string)string{p:=strings.Fields(v);if len(p)==2&&strings.EqualFold(p[0],"Bearer"){return p[1]};return ""}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,8<<10);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if err:=d.Decode(dst);err!=nil{return err};var x any;err:=d.Decode(&x);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeBillingError(w http.ResponseWriter,err error){switch{case errors.Is(err,billing.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,billing.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,billing.ErrConflict):writeError(w,409,"conflict");case errors.Is(err,billing.ErrQuotaExceeded):writeError(w,402,"quota_exceeded");case errors.Is(err,billing.ErrInvalid):writeError(w,400,"invalid_input");default:writeError(w,503,"billing_error")}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
