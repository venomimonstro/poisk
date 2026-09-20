package address

import (
	"context"
	"errors"
	"testing"

	addressbackend "github.com/venomimonstro/poisk/internal/address/backend"
)

type fakeSearchBackend struct{last addressbackend.Query}
func (f *fakeSearchBackend) Search(_ context.Context,q addressbackend.Query)(addressbackend.Result,error){f.last=q;return addressbackend.Result{},nil}

func TestSearchCapsLimit(t *testing.T){
	backend:=&fakeSearchBackend{};service:=SearchService{Backend:backend}
	if _,err:=service.Search(context.Background(),SearchRequest{Text:"Москва",RegionCode:77,Limit:500,Prefix:true});err!=nil{t.Fatal(err)}
	if backend.last.Limit!=50{t.Fatalf("limit=%d",backend.last.Limit)}
}
func TestSearchRejectsInvalidRegion(t *testing.T){
	service:=SearchService{Backend:&fakeSearchBackend{}}
	if _,err:=service.Search(context.Background(),SearchRequest{Text:"Москва",RegionCode:100});!errors.Is(err,ErrInvalidQuery){t.Fatalf("err=%v",err)}
}
func TestReverseRejectsBadCoordinatesBeforeDB(t *testing.T){
	service:=SearchService{}
	if _,err:=service.Reverse(context.Background(),91,0,500,5);!errors.Is(err,ErrInvalidQuery){t.Fatalf("err=%v",err)}
}
