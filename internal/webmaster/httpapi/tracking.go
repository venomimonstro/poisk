package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/venomimonstro/poisk/internal/webmaster"
)

type ClickRecorder interface {
	RecordClick(context.Context,string) error
}

type TrackingHandler struct { Recorder ClickRecorder }

func (h TrackingHandler) Click(w http.ResponseWriter,r *http.Request){
	if h.Recorder==nil{writeError(w,http.StatusServiceUnavailable,"tracking_unavailable");return}
	r.Body=http.MaxBytesReader(w,r.Body,4<<10)
	dec:=json.NewDecoder(r.Body);dec.DisallowUnknownFields()
	var in struct{URL string `json:"url"`}
	if err:=dec.Decode(&in);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json");return}
	var extra any;if err:=dec.Decode(&extra);err!=io.EOF{writeError(w,http.StatusBadRequest,"invalid_json");return}
	if err:=h.Recorder.RecordClick(r.Context(),in.URL);err!=nil{
		switch{
		case errors.Is(err,context.DeadlineExceeded),errors.Is(err,context.Canceled):writeError(w,http.StatusGatewayTimeout,"tracking_timeout")
		case errors.Is(err,webmaster.ErrInvalidInput):writeError(w,http.StatusBadRequest,"invalid_click")
		default:writeError(w,http.StatusInternalServerError,"tracking_error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
