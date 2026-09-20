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
	"github.com/venomimonstro/poisk/internal/webmaster"
	wmauth "github.com/venomimonstro/poisk/internal/webmaster/auth"
)

type contextKey string
const userContextKey contextKey = "webmaster-user"

type Handler struct { Service *webmaster.Service }

func (h Handler) Routes() http.Handler {
	r:=chi.NewRouter()
	r.Post("/register",h.Register)
	r.Post("/login",h.Login)
	r.Group(func(r chi.Router){
		r.Use(h.RequireAuth)
		r.Post("/logout",h.Logout)
		r.Get("/sites",h.ListSites)
		r.Post("/sites",h.AddSite)
		r.Post("/sites/{siteID}/verification",h.BeginVerification)
		r.Post("/sites/{siteID}/verify",h.CompleteVerification)
		r.Post("/sites/{siteID}/sitemaps",h.SubmitSitemap)
		r.Post("/sites/{siteID}/urls",h.SubmitURL)
		r.Get("/sites/{siteID}/url-status",h.URLStatus)
		r.Get("/sites/{siteID}/metrics",h.Metrics)
	})
	return r
}

func (h Handler) available(w http.ResponseWriter) bool {
	if h.Service!=nil { return true }
	writeError(w,http.StatusServiceUnavailable,"webmaster_unavailable")
	return false
}

func (h Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if !h.available(w) { return }
		token:=bearerToken(r.Header.Get("Authorization"))
		if token=="" { writeError(w,http.StatusUnauthorized,"unauthorized"); return }
		user,err:=h.Service.Authenticate(r.Context(),token)
		if err!=nil { writeError(w,http.StatusUnauthorized,"unauthorized"); return }
		ctx:=context.WithValue(r.Context(),userContextKey,user)
		next.ServeHTTP(w,r.WithContext(ctx))
	})
}

func (h Handler) Register(w http.ResponseWriter,r *http.Request){
	if !h.available(w) { return }
	var in struct{ Email string `json:"email"`; Password string `json:"password"` }
	if !decodeJSON(w,r,&in) { return }
	out,err:=h.Service.Register(r.Context(),in.Email,in.Password)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusCreated,out)
}

func (h Handler) Login(w http.ResponseWriter,r *http.Request){
	if !h.available(w) { return }
	var in struct{ Email string `json:"email"`; Password string `json:"password"` }
	if !decodeJSON(w,r,&in) { return }
	out,err:=h.Service.Login(r.Context(),in.Email,in.Password)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusOK,out)
}

func (h Handler) Logout(w http.ResponseWriter,r *http.Request){
	user,ok:=currentUser(r); if !ok { writeError(w,http.StatusUnauthorized,"unauthorized"); return }
	if err:=h.Service.Logout(r.Context(),user.ID,bearerToken(r.Header.Get("Authorization"))); err!=nil { writeServiceError(w,err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ListSites(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r)
	sites,err:=h.Service.Sites(r.Context(),user.ID)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusOK,map[string]any{"sites":sites})
}

func (h Handler) AddSite(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r)
	var in struct{ Origin string `json:"origin"` }
	if !decodeJSON(w,r,&in) { return }
	site,err:=h.Service.AddSite(r.Context(),user.ID,in.Origin)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusCreated,site)
}

func (h Handler) BeginVerification(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	var in struct{ Method string `json:"method"` }
	if !decodeJSON(w,r,&in) { return }
	challenge,err:=h.Service.BeginVerification(r.Context(),user.ID,siteID,strings.ToUpper(strings.TrimSpace(in.Method)))
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusCreated,challenge)
}

func (h Handler) CompleteVerification(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	var in struct{ Method string `json:"method"`; Token string `json:"token"` }
	if !decodeJSON(w,r,&in) { return }
	if err:=h.Service.CompleteVerification(r.Context(),user.ID,siteID,strings.ToUpper(strings.TrimSpace(in.Method)),strings.TrimSpace(in.Token)); err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusOK,map[string]any{"verified":true})
}

func (h Handler) SubmitSitemap(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	var in struct{ URL string `json:"url"` }
	if !decodeJSON(w,r,&in) { return }
	id,err:=h.Service.SubmitSitemap(r.Context(),user.ID,siteID,in.URL)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusAccepted,map[string]any{"sitemap_id":id})
}

func (h Handler) SubmitURL(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	var in struct{ URL string `json:"url"`; Operation string `json:"operation"` }
	if !decodeJSON(w,r,&in) { return }
	id,err:=h.Service.SubmitURL(r.Context(),user.ID,siteID,in.URL,in.Operation)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusAccepted,map[string]any{"request_id":id})
}

func (h Handler) URLStatus(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	out,err:=h.Service.URLStatus(r.Context(),user.ID,siteID,r.URL.Query().Get("url"))
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusOK,out)
}

func (h Handler) Metrics(w http.ResponseWriter,r *http.Request){
	user,_:=currentUser(r); siteID,ok:=siteIDParam(w,r); if !ok { return }
	from,err:=parseDate(r.URL.Query().Get("from")); if err!=nil { writeError(w,http.StatusBadRequest,"invalid_from"); return }
	to,err:=parseDate(r.URL.Query().Get("to")); if err!=nil { writeError(w,http.StatusBadRequest,"invalid_to"); return }
	out,err:=h.Service.Metrics(r.Context(),user.ID,siteID,from,to)
	if err!=nil { writeServiceError(w,err); return }
	writeJSON(w,http.StatusOK,out)
}

func currentUser(r *http.Request)(webmaster.User,bool){ user,ok:=r.Context().Value(userContextKey).(webmaster.User); return user,ok }
func bearerToken(v string) string { parts:=strings.Fields(v); if len(parts)!=2 || !strings.EqualFold(parts[0],"Bearer") { return "" }; return parts[1] }
func siteIDParam(w http.ResponseWriter,r *http.Request)(int64,bool){ id,err:=strconv.ParseInt(chi.URLParam(r,"siteID"),10,64); if err!=nil || id<=0 { writeError(w,http.StatusBadRequest,"invalid_site_id"); return 0,false }; return id,true }
func parseDate(v string)(time.Time,error){ if strings.TrimSpace(v)=="" { return time.Time{},nil }; return time.Parse("2006-01-02",v) }

func decodeJSON(w http.ResponseWriter,r *http.Request,dst any) bool {
	r.Body=http.MaxBytesReader(w,r.Body,64<<10)
	dec:=json.NewDecoder(r.Body); dec.DisallowUnknownFields()
	if err:=dec.Decode(dst); err!=nil { writeError(w,http.StatusBadRequest,"invalid_json"); return false }
	var extra any
	if err:=dec.Decode(&extra); err!=io.EOF { writeError(w,http.StatusBadRequest,"invalid_json"); return false }
	return true
}

func writeServiceError(w http.ResponseWriter,err error){
	switch {
	case errors.Is(err,webmaster.ErrUnauthorized): writeError(w,http.StatusUnauthorized,"unauthorized")
	case errors.Is(err,webmaster.ErrNotFound): writeError(w,http.StatusNotFound,"not_found")
	case errors.Is(err,webmaster.ErrNotVerified): writeError(w,http.StatusForbidden,"site_not_verified")
	case errors.Is(err,webmaster.ErrConflict): writeError(w,http.StatusConflict,"conflict")
	case errors.Is(err,webmaster.ErrInvalidInput),errors.Is(err,webmaster.ErrInvalidSiteOrigin),errors.Is(err,webmaster.ErrVerificationFailed),errors.Is(err,wmauth.ErrWeakPassword): writeError(w,http.StatusBadRequest,"invalid_request")
	case errors.Is(err,context.DeadlineExceeded),errors.Is(err,context.Canceled): writeError(w,http.StatusGatewayTimeout,"timeout")
	default: writeError(w,http.StatusInternalServerError,"webmaster_error")
	}
}
func writeJSON(w http.ResponseWriter,status int,v any){ w.Header().Set("Content-Type","application/json; charset=utf-8"); w.Header().Set("X-Content-Type-Options","nosniff"); w.WriteHeader(status); _=json.NewEncoder(w).Encode(v) }
func writeError(w http.ResponseWriter,status int,code string){ writeJSON(w,status,map[string]string{"error":code}) }
