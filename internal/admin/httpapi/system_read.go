package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/venomimonstro/poisk/internal/admin"
)

func (h Handler) Recovery(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};result,err:=h.Service.RecoveryReadiness(r.Context(),session);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"recovery_unavailable");return};writeJSON(w,200,result)
}
func (h Handler) QualityLatest(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};result,err:=h.Service.LatestQualityRun(r.Context(),session);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"quality_unavailable");return};writeJSON(w,200,map[string]any{"latest":result})
}
func (h Handler) AnswerMetrics(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};hours:=24;if raw:=r.URL.Query().Get("hours");raw!=""{v,err:=strconv.Atoi(raw);if err!=nil||v<1||v>720{writeError(w,400,"invalid_hours");return};hours=v};result,err:=h.Service.AnswerMetrics(r.Context(),session,hours);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"answer_metrics_unavailable");return};writeJSON(w,200,result)
}
func (h Handler) OrganizationImports(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};limit,err:=parseLimit(r,50,200);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListOrganizationImports(r.Context(),session,r.URL.Query().Get("status"),limit);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,400,"invalid_status");default:writeError(w,503,"organization_imports_unavailable")};return};writeJSON(w,200,map[string]any{"results":rows})
}
func (h Handler) OrganizationReviews(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return};limit,err:=parseLimit(r,50,200);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListOrganizationReviews(r.Context(),session,r.URL.Query().Get("status"),limit);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,400,"invalid_status");default:writeError(w,503,"organization_reviews_unavailable")};return};writeJSON(w,200,map[string]any{"results":rows})
}
