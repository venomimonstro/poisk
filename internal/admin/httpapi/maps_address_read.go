package httpapi

import (
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/admin"
)

func (h Handler) MapState(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};result,err:=h.Service.MapState(r.Context(),session);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"map_state_unavailable");return};writeJSON(w,200,result)
}
func (h Handler) AddressData(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};result,err:=h.Service.AddressData(r.Context(),session);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"address_data_unavailable");return};writeJSON(w,200,result)
}
