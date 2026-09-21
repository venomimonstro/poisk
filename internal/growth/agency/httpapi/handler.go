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
	"github.com/venomimonstro/poisk/internal/growth/agency"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

type Authenticator interface{Authenticate(context.Context,string)(webmaster.User,error)}
type Handler struct{Auth Authenticator;Agencies *agency.Repository;Billing *billing.Repository}
type ctxKey string
const userKey ctxKey="agency-user"

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Use(h.requireAuth)
	r.Get("/",h.List);r.Post("/",h.Create)
	r.Post("/{agencyID}/members",h.AddMember)
	r.Get("/{agencyID}/sites",h.ListSites)
	r.Post("/{agencyID}/sites/{siteID}",h.GrantSite)
	r.Delete("/{agencyID}/sites/{siteID}",h.RevokeSite)
	r.Post("/{agencyID}/sites/{siteID}/sitemaps",h.SubmitSitemap)
	r.Post("/{agencyID}/sites/{siteID}/urls",h.SubmitURL)
	return r
}

func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
	if h.Auth==nil||h.Agencies==nil{writeError(w,http.StatusServiceUnavailable,"agency_unavailable");return};token:=bearer(r.Header.Get("Authorization"));if token==""{writeError(w,http.StatusUnauthorized,"unauthorized");return};user,err:=h.Auth.Authenticate(r.Context(),token);if err!=nil{writeError(w,http.StatusUnauthorized,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userKey,user)))
})}
func current(r *http.Request)(webmaster.User,bool){u,ok:=r.Context().Value(userKey).(webmaster.User);return u,ok}

func (h Handler) Create(w http.ResponseWriter,r *http.Request){u,_:=current(r);var in struct{Name string `json:"name"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};out,err:=h.Agencies.Create(r.Context(),u.ID,in.Name);if err!=nil{writeAgencyError(w,err);return};if h.Billing!=nil{_,_=h.Billing.EnsureAgencySystem(r.Context(),out.ID)};writeJSON(w,201,out)}
func (h Handler) List(w http.ResponseWriter,r *http.Request){u,_:=current(r);out,err:=h.Agencies.ListForUser(r.Context(),u.ID);if err!=nil{writeAgencyError(w,err);return};writeJSON(w,200,map[string]any{"agencies":out})}
func (h Handler) AddMember(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,ok:=pathID(r,"agencyID");if !ok{writeError(w,400,"invalid_agency_id");return};var in struct{Email string `json:"email"`;Role string `json:"role"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};if !h.capacityAllowed(w,r,agencyID,"members",2,true){return};if err:=h.Agencies.AddMemberByEmail(r.Context(),u.ID,agencyID,in.Email,in.Role);err!=nil{writeAgencyError(w,err);return};writeJSON(w,200,map[string]bool{"ok":true})}
func (h Handler) ListSites(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,ok:=pathID(r,"agencyID");if !ok{writeError(w,400,"invalid_agency_id");return};sites,err:=h.Agencies.ListSites(r.Context(),u.ID,agencyID);if err!=nil{writeAgencyError(w,err);return};writeJSON(w,200,map[string]any{"sites":sites})}
func (h Handler) GrantSite(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,aok:=pathID(r,"agencyID");siteID,sok:=pathID(r,"siteID");if !aok||!sok{writeError(w,400,"invalid_id");return};var in struct{Permission string `json:"permission"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};exists,err:=h.Agencies.HasActiveSite(r.Context(),agencyID,siteID);if err!=nil{writeAgencyError(w,err);return};if !exists&&!h.capacityAllowed(w,r,agencyID,"sites",3,false){return};if err:=h.Agencies.GrantSite(r.Context(),u.ID,agencyID,siteID,in.Permission);err!=nil{writeAgencyError(w,err);return};writeJSON(w,200,map[string]bool{"ok":true})}
func (h Handler) RevokeSite(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,aok:=pathID(r,"agencyID");siteID,sok:=pathID(r,"siteID");if !aok||!sok{writeError(w,400,"invalid_id");return};if err:=h.Agencies.RevokeSite(r.Context(),u.ID,agencyID,siteID);err!=nil{writeAgencyError(w,err);return};writeJSON(w,200,map[string]bool{"ok":true})}
func (h Handler) SubmitSitemap(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,aok:=pathID(r,"agencyID");siteID,sok:=pathID(r,"siteID");if !aok||!sok{writeError(w,400,"invalid_id");return};var in struct{URL string `json:"url"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};if !h.consumeManaged(w,r,agencyID){return};id,err:=h.Agencies.SubmitSitemap(r.Context(),u.ID,agencyID,siteID,in.URL);if err!=nil{writeAgencyError(w,err);return};writeJSON(w,202,map[string]int64{"sitemap_id":id})}
func (h Handler) SubmitURL(w http.ResponseWriter,r *http.Request){u,_:=current(r);agencyID,aok:=pathID(r,"agencyID");siteID,sok:=pathID(r,"siteID");if !aok||!sok{writeError(w,400,"invalid_id");return};var in struct{URL string `json:"url"`;Operation string `json:"operation"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};if !h.consumeManaged(w,r,agencyID){return};id,err:=h.Agencies.SubmitURL(r.Context(),u.ID,agencyID,siteID,in.URL,in.Operation);if err!=nil{writeAgencyError(w,err);return};writeJSON(w,202,map[string]int64{"request_id":id})}

func (h Handler) agencyAccount(w http.ResponseWriter,r *http.Request,agencyID int64)(int64,bool){if h.Billing==nil{return 0,true};id,err:=h.Billing.EnsureAgencySystem(r.Context(),agencyID);if err!=nil{writeBillingError(w,err);return 0,false};return id,true}
func (h Handler) capacityAllowed(w http.ResponseWriter,r *http.Request,agencyID int64,metric string,freeLimit int64,members bool)bool{if h.Billing==nil{return true};accountID,ok:=h.agencyAccount(w,r,agencyID);if !ok{return false};q,err:=h.Billing.EffectiveQuota(r.Context(),accountID,"AGENCY",metric,freeLimit,time.Now());if err!=nil{writeBillingError(w,err);return false};cap,err:=h.Agencies.Capacity(r.Context(),agencyID);if err!=nil{writeAgencyError(w,err);return false};current:=cap.Sites;if members{current=cap.Members};if current>=q.Limit{writeError(w,http.StatusPaymentRequired,metric+"_limit_reached");return false};return true}
func (h Handler) consumeManaged(w http.ResponseWriter,r *http.Request,agencyID int64)bool{if h.Billing==nil{return true};accountID,ok:=h.agencyAccount(w,r,agencyID);if !ok{return false};_,_,err:=h.Billing.ConsumeEffective(r.Context(),accountID,"AGENCY","managed_requests_month",100,1,time.Now());if err!=nil{writeBillingError(w,err);return false};return true}

func pathID(r *http.Request,name string)(int64,bool){v,err:=strconv.ParseInt(chi.URLParam(r,name),10,64);return v,err==nil&&v>0}
func bearer(v string)string{parts:=strings.Fields(v);if len(parts)==2&&strings.EqualFold(parts[0],"Bearer"){return parts[1]};return ""}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,16<<10);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if err:=d.Decode(dst);err!=nil{return err};var extra any;err:=d.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeBillingError(w http.ResponseWriter,err error){switch{case errors.Is(err,billing.ErrQuotaExceeded):writeError(w,http.StatusPaymentRequired,"usage_limit_reached");case errors.Is(err,billing.ErrInvalid):writeError(w,http.StatusBadRequest,"invalid_billing_request");default:writeError(w,http.StatusServiceUnavailable,"billing_unavailable")}}
func writeAgencyError(w http.ResponseWriter,err error){switch{case errors.Is(err,agency.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,agency.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,agency.ErrInvalid):writeError(w,400,"invalid_input");default:writeError(w,503,"agency_error")}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
