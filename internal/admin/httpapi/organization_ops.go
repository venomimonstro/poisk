package httpapi

import (
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/admin"
)

type orgReviewPreviewRequest struct{ReviewID int64 `json:"review_id"`;Decision string `json:"decision"`;Reason string `json:"reason"`}
type orgReviewApplyRequest struct{PreviewToken string `json:"preview_token"`}

func (h Handler) OrganizationReviewPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in orgReviewPreviewRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,400,"invalid_json");return};preview,err:=h.Service.PreviewOrganizationReview(r.Context(),session,in.ReviewID,in.Decision,in.Reason);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"review_not_found");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,409,"review_not_open");default:writeError(w,400,"invalid_review_decision")};return};writeJSON(w,200,preview)
}
func (h Handler) OrganizationReviewApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in orgReviewApplyRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};row,err:=h.Service.ApplyOrganizationReview(r.Context(),session,in.PreviewToken);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"review_not_found");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,409,"preview_invalid_or_expired");default:writeError(w,503,"review_apply_failed")};return};writeJSON(w,200,row)
}
