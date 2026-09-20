package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/venomimonstro/poisk/internal/geo"
)

type Handler struct{ Service *geo.Service }

func (h Handler) Search(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"geo_unavailable");return}
	q:=r.URL.Query()
	req:=geo.Request{Text:q.Get("q"),CityKey:q.Get("city"),CategoryKey:q.Get("category")}
	if value:=strings.TrimSpace(q.Get("limit"));value!=""{v,err:=strconv.Atoi(value);if err!=nil||v<=0{writeError(w,http.StatusBadRequest,"invalid_limit");return};req.Limit=v}
	latRaw,lonRaw:=strings.TrimSpace(q.Get("lat")),strings.TrimSpace(q.Get("lon"))
	if latRaw!=""||lonRaw!=""{
		if latRaw==""||lonRaw==""{writeError(w,http.StatusBadRequest,"invalid_location");return}
		lat,err1:=strconv.ParseFloat(latRaw,64);lon,err2:=strconv.ParseFloat(lonRaw,64);if err1!=nil||err2!=nil{writeError(w,http.StatusBadRequest,"invalid_location");return}
		req.Latitude=&lat;req.Longitude=&lon
		if radiusRaw:=strings.TrimSpace(q.Get("radius"));radiusRaw!=""{radius,err:=strconv.Atoi(radiusRaw);if err!=nil||radius<=0{writeError(w,http.StatusBadRequest,"invalid_radius");return};req.RadiusMeters=radius}
	}
	result,err:=h.Service.Search(r.Context(),req)
	if err!=nil{
		if errors.Is(err,geo.ErrInvalidQuery){writeError(w,http.StatusBadRequest,"invalid_geo_query");return}
		writeError(w,http.StatusServiceUnavailable,"geo_unavailable");return
	}
	w.Header().Set("Content-Type","application/json; charset=utf-8")
	w.Header().Set("Cache-Control","private, max-age=15")
	w.Header().Set("X-Content-Type-Options","nosniff")
	_ = json.NewEncoder(w).Encode(result)
}

func writeError(w http.ResponseWriter,status int,code string){
	w.Header().Set("Content-Type","application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options","nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error":code})
}
