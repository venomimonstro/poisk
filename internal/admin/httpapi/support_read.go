package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/venomimonstro/poisk/internal/admin"
)

func parseLimit(r *http.Request,def,max int)(int,error){
	limit:=def
	if raw:=r.URL.Query().Get("limit");raw!=""{value,err:=strconv.Atoi(raw);if err!=nil||value<1||value>max{return 0,errors.New("invalid limit")};limit=value}
	return limit,nil
}

func (h Handler) ConsumerUsers(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return}
	limit,err:=parseLimit(r,50,100);if err!=nil{writeError(w,400,"invalid_limit");return}
	rows,err:=h.Service.ListConsumerUsers(r.Context(),session,r.URL.Query().Get("q"),limit);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"users_unavailable");return};writeJSON(w,200,map[string]any{"results":rows})
}

func (h Handler) ConsumerSessions(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return}
	userID,err:=strconv.ParseInt(r.URL.Query().Get("user_id"),10,64);if err!=nil||userID<=0{writeError(w,400,"invalid_user_id");return};limit,err:=parseLimit(r,20,100);if err!=nil{writeError(w,400,"invalid_limit");return}
	rows,err:=h.Service.ListConsumerSessions(r.Context(),session,userID,limit);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"sessions_unavailable");return};writeJSON(w,200,map[string]any{"results":rows})
}

func (h Handler) ConsumerSecurityEvents(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return}
	var userID int64;if raw:=r.URL.Query().Get("user_id");raw!=""{value,err:=strconv.ParseInt(raw,10,64);if err!=nil||value<=0{writeError(w,400,"invalid_user_id");return};userID=value}
	limit,err:=parseLimit(r,50,200);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListConsumerSecurityEvents(r.Context(),session,userID,limit);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"security_events_unavailable");return};writeJSON(w,200,map[string]any{"results":rows})
}

func (h Handler) WebmasterSites(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return}
	limit,err:=parseLimit(r,50,100);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListWebmasterSites(r.Context(),session,r.URL.Query().Get("q"),limit);if err!=nil{if errors.Is(err,admin.ErrForbidden){writeError(w,403,"forbidden");return};writeError(w,503,"webmaster_sites_unavailable");return};writeJSON(w,200,map[string]any{"results":rows})
}

func (h Handler) BillingInvoices(w http.ResponseWriter,r *http.Request){
	if h.Service==nil{writeError(w,503,"admin_unavailable");return};session,ok:=SessionFromContext(r.Context());if !ok{writeError(w,401,"unauthorized");return}
	limit,err:=parseLimit(r,50,100);if err!=nil{writeError(w,400,"invalid_limit");return};rows,err:=h.Service.ListBillingInvoices(r.Context(),session,r.URL.Query().Get("status"),limit);if err!=nil{switch{case errors.Is(err,admin.ErrForbidden):writeError(w,403,"forbidden");case errors.Is(err,admin.ErrInvalidCredential):writeError(w,400,"invalid_status");default:writeError(w,503,"billing_invoices_unavailable")};return};writeJSON(w,200,map[string]any{"results":rows})
}
