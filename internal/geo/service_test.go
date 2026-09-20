package geo

import (
	"context"
	"errors"
	"testing"

	"github.com/venomimonstro/poisk/internal/geo/backend"
)

type fakeBackend struct{ result backend.Result; err error; query backend.Query }
func (f *fakeBackend) Search(_ context.Context,q backend.Query)(backend.Result,error){f.query=q;return f.result,f.err}

func TestSearchRejectsInvalidLocation(t *testing.T){
	lat:=91.0;lon:=37.0
	service:=Service{Backend:&fakeBackend{}}
	if _,err:=service.Search(context.Background(),Request{Latitude:&lat,Longitude:&lon});!errors.Is(err,ErrInvalidQuery){t.Fatalf("expected invalid query, got %v",err)}
}

func TestSearchRequiresAtLeastOneSelector(t *testing.T){
	service:=Service{Backend:&fakeBackend{}}
	if _,err:=service.Search(context.Background(),Request{});!errors.Is(err,ErrInvalidQuery){t.Fatalf("expected invalid query, got %v",err)}
}

func TestNearbyRanksCloserHitAhead(t *testing.T){
	lat,lon:=55.75,37.62
	backendFake:=&fakeBackend{result:backend.Result{Total:2,Hits:[]backend.Hit{
		{ID:1,Score:10,Name:"Far",HasLocation:true,Latitude:55.80,Longitude:37.62,QualityScore:80,SourceCount:1},
		{ID:2,Score:10,Name:"Near",HasLocation:true,Latitude:55.751,Longitude:37.62,QualityScore:80,SourceCount:1},
	}}}
	service:=Service{Backend:backendFake}
	result,err:=service.Search(context.Background(),Request{Text:"аптека",Latitude:&lat,Longitude:&lon,RadiusMeters:10000,Limit:10})
	if err!=nil{t.Fatal(err)}
	if len(result.Results)!=2||result.Results[0].ID!=2{t.Fatalf("unexpected ordering %+v",result.Results)}
	if result.Results[0].DistanceMeters==nil||*result.Results[0].DistanceMeters<=0{t.Fatalf("missing distance %+v",result.Results[0])}
}

func TestSearchNormalizesFiltersAndCapsCandidates(t *testing.T){
	fake:=&fakeBackend{}
	service:=Service{Backend:fake,MaxCandidates:999}
	_,err:=service.Search(context.Background(),Request{Text:" кофе ",CityKey:"MOSCOW",CategoryKey:"CAFE",Limit:500})
	if err!=nil{t.Fatal(err)}
	if fake.query.CityKey!="moscow"||fake.query.CategoryKey!="cafe"{t.Fatalf("query=%+v",fake.query)}
	if fake.query.Limit!=200{t.Fatalf("candidate limit=%d",fake.query.Limit)}
}
