package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/venomimonstro/poisk/internal/growth/widget"
	"github.com/venomimonstro/poisk/internal/webmaster"
)

type WebmasterAuth interface{Authenticate(context.Context,string)(webmaster.User,error)}
type ManageHandler struct{Auth WebmasterAuth;Widgets *widget.Repository}
type manageRequest struct{SiteID int64 `json:"site_id"`;Action string `json:"action"`}

func (h ManageHandler) ServeHTTP(w http.ResponseWriter,r *http.Request){
	if h.Auth==nil||h.Widgets==nil{manageError(w,http.StatusServiceUnavailable,"widget_manage_unavailable");return}
	token:=bearer(r.Header.Get("Authorization"));if token==""{manageError(w,http.StatusUnauthorized,"unauthorized");return}
	user,err:=h.Auth.Authenticate(r.Context(),token);if err!=nil{manageError(w,http.StatusUnauthorized,"unauthorized");return}
	switch r.Method{
	case http.MethodGet:
		siteID,err:=strconv.ParseInt(r.URL.Query().Get("site_id"),10,64);if err!=nil||siteID<=0{manageError(w,http.StatusBadRequest,"invalid_site_id");return}
		if r.URL.Query().Get("analytics")=="1"{days,_:=strconv.Atoi(r.URL.Query().Get("days"));usage,err:=h.Widgets.Usage(r.Context(),user.ID,siteID,days);if err!=nil{manageError(w,http.StatusForbidden,"forbidden");return};manageJSON(w,http.StatusOK,map[string]any{"days":usage});return}
		cfg,err:=h.Widgets.Owned(r.Context(),user.ID,siteID);if err!=nil{manageError(w,http.StatusNotFound,"widget_not_found");return};manageJSON(w,http.StatusOK,cfg)
	case http.MethodPost:
		var in manageRequest;if err:=manageDecode(w,r,&in,4<<10);err!=nil||in.SiteID<=0{manageError(w,http.StatusBadRequest,"invalid_request");return}
		switch strings.ToUpper(strings.TrimSpace(in.Action)){
		case "ENSURE": cfg,err:=h.Widgets.Ensure(r.Context(),user.ID,in.SiteID);if err!=nil{manageError(w,http.StatusForbidden,"site_not_verified_or_forbidden");return};manageJSON(w,http.StatusOK,cfg)
		case "ROTATE": cfg,err:=h.Widgets.Rotate(r.Context(),user.ID,in.SiteID);if err!=nil{manageError(w,http.StatusNotFound,"widget_not_found");return};manageJSON(w,http.StatusOK,cfg)
		case "REVOKE": if err:=h.Widgets.Revoke(r.Context(),user.ID,in.SiteID);err!=nil{manageError(w,http.StatusNotFound,"widget_not_found");return};manageJSON(w,http.StatusOK,map[string]bool{"ok":true})
		default: manageError(w,http.StatusBadRequest,"invalid_action")
		}
	default:w.Header().Set("Allow","GET, POST");manageError(w,http.StatusMethodNotAllowed,"method_not_allowed")
	}
}
func bearer(raw string)string{parts:=strings.Fields(raw);if len(parts)==2&&strings.EqualFold(parts[0],"Bearer"){return parts[1]};return ""}
func manageDecode(w http.ResponseWriter,r *http.Request,dst any,max int64)error{r.Body=http.MaxBytesReader(w,r.Body,max);dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields();if err:=dec.Decode(dst);err!=nil{return err};var extra any;err:=dec.Decode(&extra);if errors.Is(err,io.EOF){return nil};if err==nil{return errors.New("trailing JSON")};return err}
func manageJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func manageError(w http.ResponseWriter,status int,code string){manageJSON(w,status,map[string]string{"error":code})}
