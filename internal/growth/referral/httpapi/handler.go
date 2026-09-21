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
	"github.com/venomimonstro/poisk/internal/growth/referral"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

type Authenticator interface{Authenticate(context.Context,string)(webmaster.User,error)}
type Handler struct{Auth Authenticator;Referrals *referral.Repository}
type key string
const userKey key="referral-user"

func (h Handler) PublicRoutes()http.Handler{r:=chi.NewRouter();r.Post("/start",h.Start);r.Post("/complete",h.Complete);return r}
func (h Handler) OwnerRoutes()http.Handler{r:=chi.NewRouter();r.Use(h.requireAuth);r.Post("/",h.Create);r.Delete("/{referralID}",h.Revoke);r.Get("/analytics",h.Analytics);return r}
func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Auth==nil||h.Referrals==nil{writeError(w,503,"referral_unavailable");return};token:=bearer(r.Header.Get("Authorization"));if token==""{writeError(w,401,"unauthorized");return};u,err:=h.Auth.Authenticate(r.Context(),token);if err!=nil{writeError(w,401,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userKey,u)))})}
func current(r *http.Request)(webmaster.User,bool){u,ok:=r.Context().Value(userKey).(webmaster.User);return u,ok}

func (h Handler) Start(w http.ResponseWriter,r *http.Request){if h.Referrals==nil{writeError(w,503,"referral_unavailable");return};var in struct{Code string `json:"code"`;Campaign string `json:"campaign"`;Flow string `json:"flow"`;LandingPath string `json:"landing_path"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};out,err:=h.Referrals.Start(r.Context(),in.Code,in.Campaign,in.Flow,in.LandingPath);if err!=nil{writeReferralError(w,err);return};writeJSON(w,201,out)}
func (h Handler) Complete(w http.ResponseWriter,r *http.Request){if h.Referrals==nil{writeError(w,503,"referral_unavailable");return};var in struct{Token string `json:"token"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};ok,err:=h.Referrals.Complete(r.Context(),in.Token);if err!=nil{writeReferralError(w,err);return};writeJSON(w,200,map[string]bool{"converted":ok})}
func (h Handler) Create(w http.ResponseWriter,r *http.Request){u,_:=current(r);var in struct{Campaign string `json:"campaign"`;TTLHours int `json:"ttl_hours"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};ttl:=time.Duration(in.TTLHours)*time.Hour;out,err:=h.Referrals.Create(r.Context(),u.ID,in.Campaign,ttl);if err!=nil{writeReferralError(w,err);return};writeJSON(w,201,out)}
func (h Handler) Revoke(w http.ResponseWriter,r *http.Request){u,_:=current(r);id,err:=strconv.ParseInt(chi.URLParam(r,"referralID"),10,64);if err!=nil||id<=0{writeError(w,400,"invalid_referral_id");return};if err=h.Referrals.Revoke(r.Context(),u.ID,id);err!=nil{writeReferralError(w,err);return};writeJSON(w,200,map[string]bool{"ok":true})}
func (h Handler) Analytics(w http.ResponseWriter,r *http.Request){u,_:=current(r);days,_:=strconv.Atoi(r.URL.Query().Get("days"));out,err:=h.Referrals.DailyForUser(r.Context(),u.ID,days);if err!=nil{writeReferralError(w,err);return};writeJSON(w,200,map[string]any{"days":out})}

func bearer(v string)string{p:=strings.Fields(v);if len(p)==2&&strings.EqualFold(p[0],"Bearer"){return p[1]};return ""}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,8<<10);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if err:=d.Decode(dst);err!=nil{return err};var extra any;err:=d.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeReferralError(w http.ResponseWriter,err error){switch{case errors.Is(err,referral.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,referral.ErrExpired):writeError(w,410,"expired");case errors.Is(err,referral.ErrInvalid):writeError(w,400,"invalid_input");default:writeError(w,503,"referral_error")}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
