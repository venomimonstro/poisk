package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/venomimonstro/poisk/internal/webmaster"
)

func TestRegisterUnavailableWithoutService(t *testing.T){
	h:=Handler{}
	rr:=httptest.NewRecorder()
	h.Register(rr,httptest.NewRequest(http.MethodPost,"/api/webmaster/register",strings.NewReader(`{"email":"a@b.com","password":"long-enough-password"}`)))
	if rr.Code!=http.StatusServiceUnavailable{t.Fatalf("status=%d",rr.Code)}
}

func TestProtectedRouteRequiresBearerToken(t *testing.T){
	h:=Handler{Service:&webmaster.Service{}}
	wrapped:=h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusOK)}))
	rr:=httptest.NewRecorder();wrapped.ServeHTTP(rr,httptest.NewRequest(http.MethodGet,"/",nil))
	if rr.Code!=http.StatusUnauthorized{t.Fatalf("status=%d",rr.Code)}
}

func TestRegisterRejectsTrailingJSON(t *testing.T){
	h:=Handler{Service:&webmaster.Service{}}
	rr:=httptest.NewRecorder()
	body:="{\"email\":\"a@b.com\",\"password\":\"long-enough-password\"}{\"x\":1}"
	h.Register(rr,httptest.NewRequest(http.MethodPost,"/api/webmaster/register",strings.NewReader(body)))
	if rr.Code!=http.StatusBadRequest{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
}

type clickRecorderFake struct{ calls int; err error; url string }
func (f *clickRecorderFake) RecordClick(_ context.Context,url string)error{f.calls++;f.url=url;return f.err}

func TestClickTrackingAcceptsSingleJSONObject(t *testing.T){
	recorder:=&clickRecorderFake{}
	h:=TrackingHandler{Recorder:recorder}
	rr:=httptest.NewRecorder()
	h.Click(rr,httptest.NewRequest(http.MethodPost,"/api/click",strings.NewReader(`{"url":"https://example.com/a"}`)))
	if rr.Code!=http.StatusNoContent || recorder.calls!=1{t.Fatalf("status=%d calls=%d",rr.Code,recorder.calls)}
}

func TestClickTrackingRejectsBadPayloadAndTimeout(t *testing.T){
	h:=TrackingHandler{Recorder:&clickRecorderFake{}}
	rr:=httptest.NewRecorder();h.Click(rr,httptest.NewRequest(http.MethodPost,"/api/click",strings.NewReader(`{"url":"x"}{"url":"y"}`)))
	if rr.Code!=http.StatusBadRequest{t.Fatalf("trailing status=%d",rr.Code)}
	h=TrackingHandler{Recorder:&clickRecorderFake{err:context.DeadlineExceeded}}
	rr=httptest.NewRecorder();h.Click(rr,httptest.NewRequest(http.MethodPost,"/api/click",strings.NewReader(`{"url":"https://example.com"}`)))
	if rr.Code!=http.StatusGatewayTimeout{t.Fatalf("timeout status=%d",rr.Code)}
	if !errors.Is(context.DeadlineExceeded,context.DeadlineExceeded){t.Fatal("unreachable")}
}
