package httpapi

import (
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/admin"
)

type domainPreviewRequest struct {
	DomainID int64  `json:"domain_id"`
	Status   string `json:"status"`
	Policy   string `json:"policy"`
}
type domainApplyRequest struct { PreviewToken string `json:"preview_token"` }

func (h Handler) DomainPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in domainPreviewRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	preview,err:=h.Service.PreviewDomainMutation(r.Context(),session,in.DomainID,in.Status,in.Policy)
	if err!=nil{switch{case errors.Is(err,admin.ErrDomainNotFound):writeError(w,http.StatusNotFound,"domain_not_found");case errors.Is(err,admin.ErrForbidden):writeError(w,http.StatusForbidden,"forbidden");default:writeError(w,http.StatusBadRequest,"invalid_domain_mutation")};return}
	writeJSON(w,http.StatusOK,preview)
}

func (h Handler) DomainApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in domainApplyRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	result,err:=h.Service.ApplyDomainMutation(r.Context(),session,in.PreviewToken)
	if err!=nil{switch{case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,http.StatusConflict,"preview_invalid_or_expired");case errors.Is(err,admin.ErrDomainNotFound):writeError(w,http.StatusNotFound,"domain_not_found");case errors.Is(err,admin.ErrForbidden):writeError(w,http.StatusForbidden,"forbidden");default:writeError(w,http.StatusServiceUnavailable,"domain_apply_failed")};return}
	writeJSON(w,http.StatusOK,result)
}
