package httpapi

import "net/http"

func (h Handler) MailGatewayHealth(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	health,err:=h.Service.MailGatewayHealth(r.Context(),session);if err!=nil{writeError(w,http.StatusServiceUnavailable,"mail_gateway_unavailable");return};writeJSON(w,http.StatusOK,health)
}
