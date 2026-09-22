package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/venomimonstro/poisk/internal/datahub"
)

type Handler struct{Hub *datahub.Repository}

func (h Handler) Routes()http.Handler{
	r:=chi.NewRouter();r.Get("/page",h.Page);r.Get("/organizations",h.Organizations);r.Get("/websites",h.Websites);r.Get("/trends",h.Trends);r.Get("/sitemap",h.Sitemap);return r
}
func (h Handler) available(w http.ResponseWriter)bool{if h.Hub!=nil{return true};writeError(w,503,"datahub_unavailable");return false}
func (h Handler) Page(w http.ResponseWriter,r *http.Request){if !h.available(w){return};p,err:=h.Hub.PublishedPage(r.Context(),r.URL.Query().Get("path"));if errors.Is(err,pgx.ErrNoRows){writeError(w,404,"not_found");return};if err!=nil{writeError(w,503,"datahub_error");return};writeJSON(w,200,map[string]any{"page":p,"seo":map[string]string{"canonical":p.CanonicalPath,"robots":"index,follow"}})}
func (h Handler) Organizations(w http.ResponseWriter,r *http.Request){if !h.available(w){return};after,ok:=intParam(r,"after",0);if !ok{writeError(w,400,"invalid_after");return};limit,ok:=intParam(r,"limit",20);if !ok||limit<1||limit>50{writeError(w,400,"invalid_limit");return};items,next,err:=h.Hub.Organizations(r.Context(),r.URL.Query().Get("city"),r.URL.Query().Get("category"),after,limit);if err!=nil{writeError(w,503,"datahub_error");return};writeJSON(w,200,map[string]any{"items":items,"next_after":next})}
func (h Handler) Websites(w http.ResponseWriter,r *http.Request){if !h.available(w){return};after,ok:=intParam(r,"after",0);if !ok{writeError(w,400,"invalid_after");return};limit,ok:=intParam(r,"limit",20);if !ok||limit<1||limit>50{writeError(w,400,"invalid_limit");return};items,next,err:=h.Hub.Websites(r.Context(),after,limit);if err!=nil{writeError(w,503,"datahub_error");return};writeJSON(w,200,map[string]any{"items":items,"next_after":next})}
func (h Handler) Trends(w http.ResponseWriter,r *http.Request){if !h.available(w){return};limit,ok:=intParam(r,"limit",20);if !ok||limit<1||limit>100{writeError(w,400,"invalid_limit");return};var day time.Time;var err error;if raw:=strings.TrimSpace(r.URL.Query().Get("day"));raw!=""{day,err=time.Parse("2006-01-02",raw);if err!=nil{writeError(w,400,"invalid_day");return}};items,err:=h.Hub.Trends(r.Context(),day,limit);if err!=nil{writeError(w,503,"datahub_error");return};writeJSON(w,200,map[string]any{"items":items})}
func (h Handler) Sitemap(w http.ResponseWriter,r *http.Request){if !h.available(w){return};after,ok:=intParam(r,"after",0);if !ok{writeError(w,400,"invalid_after");return};limit,ok:=intParam(r,"limit",200);if !ok||limit<1||limit>500{writeError(w,400,"invalid_limit");return};items,next,err:=h.Hub.Sitemap(r.Context(),after,limit);if err!=nil{writeError(w,503,"datahub_error");return};writeJSON(w,200,map[string]any{"items":items,"next_after":next})}
func intParam(r *http.Request,name string,def int)(int,bool){raw:=strings.TrimSpace(r.URL.Query().Get(name));if raw==""{return def,true};v,err:=strconv.Atoi(raw);return v,err==nil&&v>=0}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","public, max-age=60, stale-while-revalidate=300");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func writeError(w http.ResponseWriter,status int,code string){w.Header().Set("Cache-Control","no-store");writeJSON(w,status,map[string]string{"error":code})}
