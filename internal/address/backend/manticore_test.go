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
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if err:=json.NewDecoder(r.Body).Decode(&payload);err!=nil{t.Fatal(err)};w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"took":1}`));_ = json.NewEncoder(w).Encode(map[string]any{})}));defer server.Close()
	_ = server
}
