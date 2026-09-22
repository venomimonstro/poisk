package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/venomimonstro/poisk/internal/identity"
	identityhttp "github.com/venomimonstro/poisk/internal/identity/httpapi"
	"github.com/venomimonstro/poisk/internal/reviews"
)

type Handler struct{Repo reviews.Repository;Identity *identity.Service}

type authed struct{User identity.User;Session identity.Session}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter()
	r.Get("/places/{placeID}/reviews",h.List)
	r.Get("/places/{placeID}/rating",h.Rating)
	r.Group(func(p chi.Router){
		p.Use(h.requireAuth)
		p.With(h.requireCSRF).Put("/places/{placeID}/review",h.Upsert)
		p.With(h.requireCSRF).Delete("/reviews/{reviewID}",h.Delete)
		p.With(h.requireCSRF).Post("/reviews/{reviewID}/report",h.Report)
		p.With(h.requireCSRF).Put("/reviews/{reviewID}/reply",h.Reply)
	})
	return r
}

func parseID(raw string)(int64,error){v,err:=strconv.ParseInt(raw,10,64);if err!=nil||v<=0{return 0,reviews.ErrInvalid};return v,nil}
func decode(w http.ResponseWriter,r *http.Request,dst any)error{r.Body=http.MaxBytesReader(w,r.Body,16<<10);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing json")};return err}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}

func (h Handler) requireAuth(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
	if h.Identity==nil{writeError(w,503,"account_unavailable");return};cookie,err:=r.Cookie(identityhttp.SessionCookieName);if err!=nil||cookie.Value==""{writeError(w,401,"unauthorized");return};u,s,err:=h.Identity.Authenticate(r.Context(),cookie.Value);if err!=nil{writeError(w,401,"unauthorized");return};ctx:=r.Context();ctx=withAuth(ctx,authed{User:u,Session:s});next.ServeHTTP(w,r.WithContext(ctx))
})}
func (h Handler) requireCSRF(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok||!identity.VerifyCSRF(a.Session,r.Header.Get("X-CSRF-Token")){writeError(w,403,"csrf_failed");return};next.ServeHTTP(w,r)})}

func (h Handler) List(w http.ResponseWriter,r *http.Request){placeID,err:=parseID(chi.URLParam(r,"placeID"));if err!=nil{writeError(w,400,"invalid_place_id");return};limit:=20;if raw:=r.URL.Query().Get("limit");raw!=""{v,e:=strconv.Atoi(raw);if e!=nil||v<1||v>100{writeError(w,400,"invalid_limit");return};limit=v};var before int64;if raw:=r.URL.Query().Get("before_id");raw!=""{before,err=strconv.ParseInt(raw,10,64);if err!=nil||before<=0{writeError(w,400,"invalid_cursor");return}};items,err:=h.Repo.ListPublic(r.Context(),placeID,limit,before);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,map[string]any{"reviews":items})}
func (h Handler) Rating(w http.ResponseWriter,r *http.Request){placeID,err:=parseID(chi.URLParam(r,"placeID"));if err!=nil{writeError(w,400,"invalid_place_id");return};stats,err:=h.Repo.Stats(r.Context(),placeID);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,stats)}

func (h Handler) Upsert(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok{writeError(w,401,"unauthorized");return};placeID,err:=parseID(chi.URLParam(r,"placeID"));if err!=nil{writeError(w,400,"invalid_place_id");return};var in struct{Rating int `json:"rating"`;Body string `json:"body"`};if err:=decode(w,r,&in);err!=nil{writeError(w,400,"invalid_json");return};if err:=h.Repo.ConsumeAction(r.Context(),a.User.ID,"WRITE",time.Now().UTC());err!=nil{writeRepoError(w,err);return};item,err:=h.Repo.Upsert(r.Context(),a.User.ID,placeID,in.Rating,in.Body);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,item)}
func (h Handler) Delete(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok{writeError(w,401,"unauthorized");return};reviewID,err:=parseID(chi.URLParam(r,"reviewID"));if err!=nil{writeError(w,400,"invalid_review_id");return};if err:=h.Repo.ConsumeAction(r.Context(),a.User.ID,"WRITE",time.Now().UTC());err!=nil{writeRepoError(w,err);return};if err:=h.Repo.SoftDelete(r.Context(),a.User.ID,reviewID);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) Report(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok{writeError(w,401,"unauthorized");return};reviewID,err:=parseID(chi.URLParam(r,"reviewID"));if err!=nil{writeError(w,400,"invalid_review_id");return};var in struct{Reason string `json:"reason"`;Details string `json:"details"`};if err:=decode(w,r,&in);err!=nil{writeError(w,400,"invalid_json");return};if err:=h.Repo.ConsumeAction(r.Context(),a.User.ID,"REPORT",time.Now().UTC());err!=nil{writeRepoError(w,err);return};if err:=h.Repo.Report(r.Context(),a.User.ID,reviewID,in.Reason,in.Details);err!=nil{writeRepoError(w,err);return};w.WriteHeader(204)}
func (h Handler) Reply(w http.ResponseWriter,r *http.Request){a,ok:=authFrom(r);if !ok{writeError(w,401,"unauthorized");return};reviewID,err:=parseID(chi.URLParam(r,"reviewID"));if err!=nil{writeError(w,400,"invalid_review_id");return};var in struct{Body string `json:"body"`};if err:=decode(w,r,&in);err!=nil{writeError(w,400,"invalid_json");return};if err:=h.Repo.ConsumeAction(r.Context(),a.User.ID,"REPLY",time.Now().UTC());err!=nil{writeRepoError(w,err);return};reply,err:=h.Repo.Reply(r.Context(),a.User.ID,reviewID,in.Body);if err!=nil{writeRepoError(w,err);return};writeJSON(w,200,reply)}

func writeRepoError(w http.ResponseWriter,err error){switch{case errors.Is(err,reviews.ErrInvalid):writeError(w,400,"invalid_request");case errors.Is(err,reviews.ErrNotFound):writeError(w,404,"not_found");case errors.Is(err,reviews.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,reviews.ErrConflict):writeError(w,409,"conflict");case errors.Is(err,reviews.ErrRateLimited):writeError(w,429,"rate_limited");default:writeError(w,503,"reviews_unavailable")}}
