package httpapi

import (
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/admin"
)

type reviewModerationPreviewRequest struct{ReviewID int64 `json:"review_id"`;Action string `json:"action"`;Reason string `json:"reason"`}
type reviewModerationApplyRequest struct{PreviewToken string `json:"preview_token"`}

func (h Handler) ReviewModerationQueue(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};limit,err:=parseLimit(r,50,200);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListReviewModeration(r.Context(),session,r.URL.Query().Get("status"),limit);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,400,"invalid_status");default:writeError(w,503,"reviews_moderation_unavailable")};return};writeJSON(w,200,map[string]any{"results":rows})
}
func (h Handler) ReviewModerationPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in reviewModerationPreviewRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,400,"invalid_json");return};preview,err:=h.Service.PreviewReviewModeration(r.Context(),session,in.ReviewID,in.Action,in.Reason);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"review_not_found");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,409,"review_not_moderatable");default:writeError(w,400,"invalid_review_moderation")};return};writeJSON(w,200,preview)
}
func (h Handler) ReviewModerationApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in reviewModerationApplyRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};row,err:=h.Service.ApplyReviewModeration(r.Context(),session,in.PreviewToken);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"review_not_found");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,409,"preview_invalid_or_expired");default:writeError(w,503,"review_moderation_apply_failed")};return};writeJSON(w,200,row)
}
