package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchCapsLimitAndAddsRegionFilter(t *testing.T){
	var payload map[string]any
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Fatal(err)}
		w.Header().Set("Content-Type","application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"took":1,
			"timed_out":false,
			"hits":map[string]any{"total":0,"hits":[]any{}},
		})
	}))
	defer server.Close()
	client,err:=New(Config{BaseURL:server.URL,MaxResults:12});if err!=nil{t.Fatal(err)}
	if _,err:=client.Search(context.Background(),Query{Text:"Москва Тверская",RegionCode:77,Limit:999,Prefix:true});err!=nil{t.Fatal(err)}
	if payload["limit"].(float64)!=12{t.Fatalf("limit=%v",payload["limit"])}
	query:=payload["query"].(map[string]any);b:=query["bool"].(map[string]any);must:=b["must"].([]any);if len(must)<3{t.Fatalf("must=%#v",must)}
}

func TestSearchRejectsEmptyQuery(t *testing.T){
	client,err:=New(Config{BaseURL:"http://127.0.0.1:1"});if err!=nil{t.Fatal(err)}
	if _,err:=client.Search(context.Background(),Query{});err==nil{t.Fatal("expected invalid query")}
}
