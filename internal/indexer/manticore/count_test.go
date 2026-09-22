package manticore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCountDocumentsParsesRawResult(t *testing.T){
	var query string
	srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){body,_:=io.ReadAll(r.Body);query=string(body);_,_=io.WriteString(w,`[{"data":[{"documents":"12345"}],"error":"","warning":""}]`)}));defer srv.Close()
	client,err:=New(Config{BaseURL:srv.URL});if err!=nil{t.Fatal(err)}
	count,err:=client.CountDocuments(context.Background());if err!=nil{t.Fatal(err)}
	if count!=12345{t.Fatalf("count=%d",count)}
	if !strings.Contains(query,"COUNT(*)")||!strings.Contains(query,WebIndex){t.Fatalf("query=%q",query)}
}
