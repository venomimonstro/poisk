package httpapi

import "net/http"

func (h Handler) RotateCSRF(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	token,err:=h.Service.RotateCSRF(r.Context(),session);if err!=nil{writeError(w,http.StatusServiceUnavailable,"csrf_rotation_failed");return}
	writeJSON(w,http.StatusOK,map[string]string{"csrf_token":token})
}
