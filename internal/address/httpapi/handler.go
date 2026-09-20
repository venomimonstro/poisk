package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/venomimonstro/poisk/internal/address"
)

type Handler struct{Service *address.SearchService}

func (h Handler) Search(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"address_unavailable");return};q:=r.URL.Query();text:=strings.TrimSpace(q.Get("q"));region:=0;limit:=10
	if raw:=strings.TrimSpace(q.Get("region"));raw!=""{v,err:=strconv.Atoi(raw);if err!=nil||v<1||v>99{writeError(w,http.StatusBadRequest,"invalid_region");return};region=v}
	if raw:=strings.TrimSpace(q.Get("limit"));raw!=""{v,err:=strconv.Atoi(raw);if err!=nil||v<=0{writeError(w,http.StatusBadRequest,"invalid_limit");return};limit=v}
	prefix:=q.Get("mode")!="search";result,err:=h.Service.Search(r.Context(),address.SearchRequest{Text:text,RegionCode:region,Limit:limit,Prefix:prefix});if err!=nil{if errors.Is(err,address.ErrInvalidQuery){writeError(w,http.StatusBadRequest,"invalid_address_query");return};writeError(w,http.StatusServiceUnavailable,"address_unavailable");return};writeJSON(w,result)
}

func (h Handler) Forward(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"address_unavailable");return};id,err:=strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("id")),10,64);if err!=nil||id<=0{writeError(w,http.StatusBadRequest,"invalid_address_id");return};result,err:=h.Service.Forward(r.Context(),id);if err!=nil{writeError(w,http.StatusNotFound,"address_not_found");return};writeJSON(w,result)
}

func (h Handler) Reverse(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"address_unavailable");return};q:=r.URL.Query();lat,err1:=strconv.ParseFloat(strings.TrimSpace(q.Get("lat")),64);lon,err2:=strconv.ParseFloat(strings.TrimSpace(q.Get("lon")),64);if err1!=nil||err2!=nil{writeError(w,http.StatusBadRequest,"invalid_location");return};radius:=500;limit:=5
	if raw:=strings.TrimSpace(q.Get("radius"));raw!=""{v,err:=strconv.Atoi(raw);if err!=nil{writeError(w,http.StatusBadRequest,"invalid_radius");return};radius=v};if raw:=strings.TrimSpace(q.Get("limit"));raw!=""{v,err:=strconv.Atoi(raw);if err!=nil{writeError(w,http.StatusBadRequest,"invalid_limit");return};limit=v}
	result,err:=h.Service.Reverse(r.Context(),lat,lon,radius,limit);if err!=nil{if errors.Is(err,address.ErrInvalidQuery){writeError(w,http.StatusBadRequest,"invalid_reverse_query");return};writeError(w,http.StatusServiceUnavailable,"address_unavailable");return};writeJSON(w,result)
}

func writeJSON(w http.ResponseWriter,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","private, max-age=30");w.Header().Set("X-Content-Type-Options","nosniff");_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,code string){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(map[string]string{"error":code})}
