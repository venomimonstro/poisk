package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/venomimonstro/poisk/internal/admin"
	mailcore "github.com/venomimonstro/poisk/internal/mail"
)

func (h Handler) MailDeadLetters(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	limit:=50;if raw:=r.URL.Query().Get("limit");raw!=""{v,err:=strconv.Atoi(raw);if err!=nil||v<1||v>200{writeError(w,http.StatusBadRequest,"invalid_limit");return};limit=v}
	var before int64;if raw:=r.URL.Query().Get("before_id");raw!=""{v,err:=strconv.ParseInt(raw,10,64);if err!=nil||v<=0{writeError(w,http.StatusBadRequest,"invalid_cursor");return};before=v}
	items,err:=h.Service.ListMailDeadLetters(r.Context(),session,limit,before);if err!=nil{writeError(w,http.StatusServiceUnavailable,"mail_dead_unavailable");return};writeJSON(w,http.StatusOK,map[string]any{"deliveries":items})
}

func (h Handler) MailDeadRetryPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in struct{DeliveryID int64 `json:"delivery_id"`};if err:=decodeOne(w,r,&in,8<<10);err!=nil||in.DeliveryID<=0{writeError(w,http.StatusBadRequest,"invalid_request");return}
	preview,err:=h.Service.PreviewMailDeadRetry(r.Context(),session,in.DeliveryID);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden),errors.Is(err,mailcore.ErrForbidden):writeError(w,http.StatusForbidden,"not_retryable");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,http.StatusConflict,"stale_delivery");default:writeError(w,http.StatusServiceUnavailable,"mail_dead_unavailable")};return};writeJSON(w,http.StatusOK,preview)
}

func (h Handler) MailDeadRetryApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in struct{PreviewToken string `json:"preview_token"`};if err:=decodeOne(w,r,&in,8<<10);err!=nil||in.PreviewToken==""{writeError(w,http.StatusBadRequest,"invalid_request");return}
	err:=h.Service.ApplyMailDeadRetry(r.Context(),session,in.PreviewToken);if err!=nil{switch{case errors.Is(err,admin.ErrPreviewInvalid),errors.Is(err,mailcore.ErrConflict):writeError(w,http.StatusConflict,"stale_preview");case errors.Is(err,mailcore.ErrForbidden):writeError(w,http.StatusForbidden,"not_retryable");case errors.Is(err,mailcore.ErrRateLimited):writeError(w,http.StatusTooManyRequests,"retry_limit_reached");default:writeError(w,http.StatusServiceUnavailable,"mail_dead_retry_failed")};return};writeJSON(w,http.StatusOK,map[string]bool{"ok":true})
}
