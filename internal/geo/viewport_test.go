package geo

import (
	"context"
	"errors"
	"testing"

	"github.com/venomimonstro/poisk/internal/geo/backend"
)

func TestViewportRejectsInvalidBounds(t *testing.T){
	service:=Service{Backend:&fakeBackend{}}
	_,err:=service.Viewport(context.Background(),ViewportRequest{MinLatitude:10,MaxLatitude:5,MinLongitude:20,MaxLongitude:30,Zoom:8})
	if !errors.Is(err,ErrInvalidQuery){t.Fatalf("expected invalid query, got %v",err)}
}

func TestViewportClustersNearbyPoints(t *testing.T){
	fake:=&fakeBackend{result:backend.Result{Total:3,Hits:[]backend.Hit{
		{ID:1,Name:"A",HasLocation:true,Latitude:55.7500,Longitude:37.6200,Status:""},
		{ID:2,Name:"B",HasLocation:true,Latitude:55.7502,Longitude:37.6202},
		{ID:3,Name:"C",HasLocation:true,Latitude:55.90,Longitude:37.90},
	}}}
	service:=Service{Backend:fake}
	result,err:=service.Viewport(context.Background(),ViewportRequest{MinLatitude:55.6,MinLongitude:37.4,MaxLatitude:56.0,MaxLongitude:38.0,Zoom:10,Limit:200})
	if err!=nil{t.Fatal(err)}
	if len(result.Results)!=3{t.Fatalf("results=%d",len(result.Results))}
	foundPair:=false
	for _,cluster:=range result.Clusters{if cluster.Count>=2{foundPair=true}}
	if !foundPair{t.Fatalf("expected clustered nearby points: %+v",result.Clusters)}
}

func TestViewportPassesBoundingBoxToBackend(t *testing.T){
	fake:=&fakeBackend{}
	service:=Service{Backend:fake}
	_,err:=service.Viewport(context.Background(),ViewportRequest{MinLatitude:50,MinLongitude:30,MaxLatitude:60,MaxLongitude:40,Zoom:5,CityKey:"moscow"})
	if err!=nil{t.Fatal(err)}
	if fake.query.MinLatitude==nil||fake.query.MaxLongitude==nil{t.Fatalf("missing bbox %+v",fake.query)}
	if *fake.query.MinLatitude!=50||*fake.query.MaxLongitude!=40{t.Fatalf("bbox %+v",fake.query)}
}
