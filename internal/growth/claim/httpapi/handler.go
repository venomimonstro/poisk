package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/growth/claim"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

type Authenticator interface{Authenticate(context.Context,string)(webmaster.User,error)}
type Handler struct{Auth Authenticator;Claims *claim.Repository}
type key string
const userKey key="claim-user"

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Use(h.requireAuth);r.Get("/",h.List);r.Post("/{placeID}",h.Claim);r.Delete("/{placeID}",h.Revoke);return r
}
func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if h.Auth==nil||h.Claims==nil{writeError(w,503,"claim_unavailable");return};token:=bearer(r.Header.Get("Authorization"));if token==""{writeError(w,401,"unauthorized");return};u,err:=h.Auth.Authenticate(r.Context(),token);if err!=nil{writeError(w,401,"unauthorized");return};next.ServeHTTP(w,r.WithContext(context.WithValue(r.Context(),userKey,u)))})}
func current(r *http.Request)(webmaster.User,bool){u,ok:=r.Context().Value(userKey).(webmaster.User);return u,ok}
func (h Handler) Claim(w http.ResponseWriter,r *http.Request){u,_:=current(r);placeID,ok:=pathID(r);if !ok{writeError(w,400,"invalid_place_id");return};var in struct{SiteID int64 `json:"site_id"`};if decode(w,r,&in)!=nil{writeError(w,400,"invalid_json");return};out,err:=h.Claims.Claim(r.Context(),u.ID,in.SiteID,placeID);if err!=nil{writeClaimError(w,err);return};writeJSON(w,201,out)}
func (h Handler) Revoke(w http.ResponseWriter,r *http.Request){u,_:=current(r);placeID,ok:=pathID(r);if !ok{writeError(w,400,"invalid_place_id");return};if err:=h.Claims.Revoke(r.Context(),u.ID,placeID);err!=nil{writeClaimError(w,err);return};writeJSON(w,200,map[string]bool{"ok":true})}
func (h Handler) List(w http.ResponseWriter,r *http.Request){u,_:=current(r);items,err:=h.Claims.List(r.Context(),u.ID);if err!=nil{writeClaimError(w,err);return};writeJSON(w,200,map[string]any{"claims":items})}
func pathID(r *http.Request)(int64,bool){v,err:=strconv.ParseInt(chi.URLParam(r,"placeID"),10,64);return v,err==nil&&v>0}
func bearer(v string)string{p:=strings.Fields(v);if len(p)==2&&strings.EqualFold(p[0],"Bearer"){return p[1]};return ""}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,8<<10);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields();if err:=d.Decode(dst);err!=nil{return err};var extra any;err:=d.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeClaimError(w http.ResponseWriter,err error){switch{case errors.Is(err,claim.ErrForbidden):writeError(w,403,"claim_proof_failed");case errors.Is(err,claim.ErrConflict):writeError(w,409,"already_claimed");case errors.Is(err,claim.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,claim.ErrInvalid):writeError(w,400,"invalid_input");default:writeError(w,503,"claim_error")}}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
