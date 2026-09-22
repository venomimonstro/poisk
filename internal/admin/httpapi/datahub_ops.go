package httpapi

import (
	"errors"
	"net/http"

	"github.com/venomimonstro/poisk/internal/admin"
)

type dataHubPreviewRequest struct{PageID int64 `json:"page_id"`;Action string `json:"action"`;Reason string `json:"reason"`;TargetVersion int64 `json:"target_version"`}
type dataHubApplyRequest struct{PreviewToken string `json:"preview_token"`}

func (h Handler) DataHubPages(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};limit,err:=parseLimit(r,50,200);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListDataHubPages(r.Context(),session,r.URL.Query().Get("state"),limit);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,400,"invalid_state");default:writeError(w,503,"datahub_unavailable")};return};writeJSON(w,200,map[string]any{"results":rows})
}
func (h Handler) DataHubPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in dataHubPreviewRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,400,"invalid_json");return};preview,err:=h.Service.PreviewDataHubMutation(r.Context(),session,in.PageID,in.Action,in.Reason,in.TargetVersion);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"datahub_page_not_found");default:writeError(w,400,"invalid_datahub_mutation")};return};writeJSON(w,200,preview)
}
func (h Handler) DataHubApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};var in dataHubApplyRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,400,"invalid_json");return};row,err:=h.Service.ApplyDataHubMutation(r.Context(),session,in.PreviewToken);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,409,"preview_invalid_or_expired");case errors.Is(err,admin.ErrNotFound):writeError(w,404,"datahub_page_not_found");default:writeError(w,503,"datahub_apply_failed")};return};writeJSON(w,200,row)
}
