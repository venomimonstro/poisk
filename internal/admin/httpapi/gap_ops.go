package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/venomimonstro/poisk/internal/admin"
)

type gapPreviewRequest struct {
	GapID int64 `json:"gap_id"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}
type gapApplyRequest struct { PreviewToken string `json:"preview_token"` }

func (h Handler) QueryGaps(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	limit:=50;if raw:=r.URL.Query().Get("limit");raw!=""{value,err:=strconv.Atoi(raw);if err!=nil||value<1||value>200{writeError(w,http.StatusBadRequest,"invalid_limit");return};limit=value}
	rows,err:=h.Service.ListQueryGaps(r.Context(),session,r.URL.Query().Get("state"),limit)
	if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,http.StatusForbidden,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,http.StatusBadRequest,"invalid_state");default:writeError(w,http.StatusServiceUnavailable,"query_gaps_unavailable")};return}
	writeJSON(w,http.StatusOK,map[string]any{"results":rows})
}

func (h Handler) QueryGapPreview(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in gapPreviewRequest;if err:=decodeOne(w,r,&in,8<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	preview,err:=h.Service.PreviewQueryGapMutation(r.Context(),session,in.GapID,in.Action,in.Reason)
	if err!=nil{switch{case errors.Is(err,admin.ErrQueryGapNotFound):writeError(w,http.StatusNotFound,"query_gap_not_found");case errors.Is(err,admin.ErrForbidden):writeError(w,http.StatusForbidden,"forbidden");default:writeError(w,http.StatusBadRequest,"invalid_query_gap_mutation")};return}
	writeJSON(w,http.StatusOK,preview)
}

func (h Handler) QueryGapApply(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,http.StatusServiceUnavailable,"admin_unavailable");return}
	session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,http.StatusUnauthorized,"unauthorized");return}
	var in gapApplyRequest;if err:=decodeOne(w,r,&in,4<<10);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	result,err:=h.Service.ApplyQueryGapMutation(r.Context(),session,in.PreviewToken)
	if err!=nil{switch{case errors.Is(err,admin.ErrPreviewInvalid):writeError(w,http.StatusConflict,"preview_invalid_or_expired");case errors.Is(err,admin.ErrQueryGapNotFound):writeError(w,http.StatusNotFound,"query_gap_not_found");case errors.Is(err,admin.ErrForbidden):writeError(w,http.StatusForbidden,"forbidden");default:writeError(w,http.StatusServiceUnavailable,"query_gap_apply_failed")};return}
	writeJSON(w,http.StatusOK,result)
}
