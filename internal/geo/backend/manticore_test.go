package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchBuildsServerSideFilters(t *testing.T){
	var payload map[string]any
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if r.URL.Path!="/search"{t.Fatalf("path=%s",r.URL.Path)}
		if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Fatal(err)}
		w.Header().Set("Content-Type","application/json")
		response:=map[string]any{
			"took":1,"timed_out":false,
			"hits":map[string]any{"total":1,"hits":[]any{map[string]any{
				"_id":7,"_score":12,
				"_source":map[string]any{"name":"Cafe","address":"Street","city_key":"moscow","category_key":"cafe","latitude":55.75,"longitude":37.62,"has_location":true,"quality_score":80,"source_count":2},
			}}},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client,err:=New(Config{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
	lat,lon:=55.75,37.62
	result,err:=client.Search(context.Background(),Query{Text:"coffee",CityKey:"moscow",CategoryKey:"cafe",Latitude:&lat,Longitude:&lon,RadiusMeters:3000,Limit:10})
	if err!=nil{t.Fatal(err)}
	if len(result.Hits)!=1||result.Hits[0].ID!=7{t.Fatalf("result=%+v",result)}
	query,ok:=payload["query"].(map[string]any);if !ok{t.Fatalf("query=%T",payload["query"])}
	boolQuery,ok:=query["bool"].(map[string]any);if !ok{t.Fatalf("bool=%T",query["bool"])}
	must,ok:=boolQuery["must"].([]any);if !ok||len(must)<5{t.Fatalf("must=%#v",boolQuery["must"])}
}

func TestSearchCapsLimit(t *testing.T){
	var limit float64
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		limit = payload["limit"].(float64)
		w.Header().Set("Content-Type","application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"took":0,"timed_out":false,"hits":map[string]any{"total":0,"hits":[]any{}}})
	}))
	defer server.Close()
	client,err:=New(Config{BaseURL:server.URL,MaxResults:25});if err!=nil{t.Fatal(err)}
	if _,err:=client.Search(context.Background(),Query{Text:"x",Limit:1000});err!=nil{t.Fatal(err)}
	if limit!=25{t.Fatalf("limit=%v",limit)}
}
