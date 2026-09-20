package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/venomimonstro/poisk/internal/growth/widget"
	"github.com/venomimonstro/poisk/internal/platform/guard"
)

type Resolver interface{ResolvePublic(rctx context.Context,key string)(widget.Config,error)}

type Handler struct{
	Widgets *widget.Repository
	Search widget.Service
	KeyLimiter *guard.Limiter
	ClientLimiter *guard.Limiter
}

func (h Handler) ServeHTTP(w http.ResponseWriter,r *http.Request){
	if h.Widgets==nil||h.Search.Backend==nil{writeError(w,http.StatusServiceUnavailable,"widget_unavailable");return}
	if r.Method!=http.MethodPost&&r.Method!=http.MethodOptions{w.Header().Set("Allow","POST, OPTIONS");writeError(w,http.StatusMethodNotAllowed,"method_not_allowed");return}
	key:=strings.TrimSpace(r.URL.Query().Get("key"));cfg,err:=h.Widgets.ResolvePublic(r.Context(),key);if err!=nil{writeError(w,http.StatusNotFound,"widget_not_found");return}
	origin:=strings.TrimSpace(r.Header.Get("Origin"));if !originAllowed(origin,cfg.Origin){writeError(w,http.StatusForbidden,"origin_forbidden");return}
	setCORS(w,origin)
	if r.Method==http.MethodOptions{w.WriteHeader(http.StatusNoContent);return}
	client:=clientIP(r);if h.KeyLimiter!=nil&&!h.KeyLimiter.Allow(cfg.PublicKey){writeError(w,http.StatusTooManyRequests,"rate_limited");return};if h.ClientLimiter!=nil&&!h.ClientLimiter.Allow(cfg.PublicKey+"|"+client){writeError(w,http.StatusTooManyRequests,"rate_limited");return}
	var in widget.SearchRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	out,err:=h.Search.Search(r.Context(),cfg,in);if err!=nil{writeError(w,http.StatusBadRequest,"invalid_query");return}
	writeJSON(w,http.StatusOK,out)
}

func originAllowed(got,want string)bool{
	g,err:=url.Parse(got);if err!=nil||g.Scheme==""||g.Host==""||g.User!=nil||g.RawQuery!=""||g.Fragment!=""{return false}
	w,err:=url.Parse(want);if err!=nil||w.Scheme==""||w.Host==""{return false}
	return strings.EqualFold(g.Scheme,w.Scheme)&&strings.EqualFold(g.Host,w.Host)&&g.Path==""&&w.Path==""
}
func setCORS(w http.ResponseWriter,origin string){w.Header().Set("Access-Control-Allow-Origin",origin);w.Header().Set("Vary","Origin");w.Header().Set("Access-Control-Allow-Methods","POST, OPTIONS");w.Header().Set("Access-Control-Allow-Headers","Content-Type");w.Header().Set("Access-Control-Max-Age","600")}
func clientIP(r *http.Request)string{host,_,err:=net.SplitHostPort(r.RemoteAddr);if err==nil&&host!=""{return host};return r.RemoteAddr}
func decodeOne(w http.ResponseWriter,r *http.Request,dst any,max int64)error{r.Body=http.MaxBytesReader(w,r.Body,max);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing JSON")};return err}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.Header().Set("X-Content-Type-Options","nosniff");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,code string){writeJSON(w,status,map[string]string{"error":code})}
