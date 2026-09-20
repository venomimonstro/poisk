package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/venomimonstro/poisk/internal/maps"
)

type configProviderFake struct{ cfg maps.Config; err error }
func (f configProviderFake) ActiveConfig(context.Context)(maps.Config,error){return f.cfg,f.err}

func TestConfigReturnsActiveMapContract(t *testing.T){
	h:=Handler{Maps:configProviderFake{cfg:maps.Config{Version:"v1",PMTilesURL:"/maps/tiles/v1.pmtiles",StyleURL:"/maps/styles/v1.json",SourceName:"osm"}}}
	rr:=httptest.NewRecorder();h.Config(rr,httptest.NewRequest(http.MethodGet,"/api/map/config",nil))
	if rr.Code!=http.StatusOK{t.Fatalf("status=%d body=%s",rr.Code,rr.Body.String())}
	if rr.Header().Get("X-Content-Type-Options")!="nosniff"{t.Fatal("missing nosniff")}
	if rr.Header().Get("Cache-Control")==""{t.Fatal("missing cache policy")}
}

func TestConfigMapsUnavailableStatesTo503(t *testing.T){
	for _,err:=range []error{maps.ErrNoActiveMap,maps.ErrMapNotFound,maps.ErrInvalidManifest,errors.New("disk unavailable")} {
		h:=Handler{Maps:configProviderFake{err:err}}
		rr:=httptest.NewRecorder();h.Config(rr,httptest.NewRequest(http.MethodGet,"/api/map/config",nil))
		if rr.Code!=http.StatusServiceUnavailable{t.Fatalf("err=%v status=%d",err,rr.Code)}
	}
}

func TestConfigMapsDeadlineTo504(t *testing.T){
	h:=Handler{Maps:configProviderFake{err:context.DeadlineExceeded}}
	rr:=httptest.NewRecorder();h.Config(rr,httptest.NewRequest(http.MethodGet,"/api/map/config",nil))
	if rr.Code!=http.StatusGatewayTimeout{t.Fatalf("status=%d",rr.Code)}
}
