package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/venomimonstro/poisk/internal/geo"
)

func TestSearchRejectsPartialCoordinatesBeforeBackend(t *testing.T){
	h:=Handler{Service:&geo.Service{}}
	req:=httptest.NewRequest(http.MethodGet,"/api/geo/search?q=coffee&lat=55.75",nil)
	rr:=httptest.NewRecorder()
	h.Search(rr,req)
	if rr.Code!=http.StatusBadRequest{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
}

func TestViewportRejectsMissingBoundsBeforeBackend(t *testing.T){
	h:=Handler{Service:&geo.Service{}}
	req:=httptest.NewRequest(http.MethodGet,"/api/geo/viewport?min_lat=55&min_lon=37&max_lat=56&zoom=10",nil)
	rr:=httptest.NewRecorder()
	h.Viewport(rr,req)
	if rr.Code!=http.StatusBadRequest{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
}

func TestSearchRejectsInvalidLimit(t *testing.T){
	h:=Handler{Service:&geo.Service{}}
	req:=httptest.NewRequest(http.MethodGet,"/api/geo/search?q=x&limit=0",nil)
	rr:=httptest.NewRecorder()
	h.Search(rr,req)
	if rr.Code!=http.StatusBadRequest{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
}
