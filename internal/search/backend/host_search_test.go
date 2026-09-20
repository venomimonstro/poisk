package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchHostSendsExactHostFilter(t *testing.T){
	var payload map[string]any
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Fatal(err)}
		w.Header().Set("Content-Type","application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"took":1,"timed_out":false,"hits":map[string]any{"total":1,"hits":[]any{map[string]any{"_id":1,"_score":2,"_source":map[string]any{"title":"A","url":"https://shop.example/a","host":"shop.example"}}}}})
	}));defer server.Close()
	client,err:=New(Config{BaseURL:server.URL,MaxResults:20});if err!=nil{t.Fatal(err)}
	result,err:=client.SearchHost(context.Background(),"товар","shop.example",99);if err!=nil{t.Fatal(err)}
	if len(result.Hits)!=1||result.Hits[0].Host!="shop.example"{t.Fatalf("hits=%+v",result.Hits)}
	query:=payload["query"].(map[string]any);boolean:=query["bool"].(map[string]any);must:=boolean["must"].([]any);if len(must)!=2{t.Fatalf("must=%#v",must)}
	equals:=must[1].(map[string]any)["equals"].(map[string]any);if equals["host"]!="shop.example"{t.Fatalf("equals=%#v",equals)}
	if payload["limit"].(float64)!=20{t.Fatalf("limit=%v",payload["limit"])}
}
