package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/maps"
)

type ConfigProvider interface {
	ActiveConfig(context.Context)(maps.Config,error)
}

type Handler struct{ Maps ConfigProvider }

func (h Handler) Config(w http.ResponseWriter,r *http.Request){
	if h.Maps==nil{writeError(w,http.StatusServiceUnavailable,"map_unavailable");return}
	cfg,err:=h.Maps.ActiveConfig(r.Context())
	if err!=nil{
		switch{
		case errors.Is(err,context.DeadlineExceeded),errors.Is(err,context.Canceled):writeError(w,http.StatusGatewayTimeout,"map_timeout")
		case errors.Is(err,maps.ErrNoActiveMap),errors.Is(err,maps.ErrMapNotFound),errors.Is(err,maps.ErrInvalidManifest):writeError(w,http.StatusServiceUnavailable,"map_unavailable")
		default:writeError(w,http.StatusServiceUnavailable,"map_unavailable")
		}
		return
	}
	w.Header().Set("Content-Type","application/json; charset=utf-8")
	w.Header().Set("Cache-Control","public, max-age=30, stale-while-revalidate=60")
	w.Header().Set("X-Content-Type-Options","nosniff")
	_ = json.NewEncoder(w).Encode(cfg)
}

func writeError(w http.ResponseWriter,status int,code string){
	w.Header().Set("Content-Type","application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options","nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error":code})
}
