package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	querynorm "github.com/venomimonstro/poisk/internal/query"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

type fakeSearcher struct {
	response searchsvc.Response
	err      error
	request  searchsvc.Request
}

func (f *fakeSearcher) Search(_ context.Context, req searchsvc.Request) (searchsvc.Response, error) {
	f.request = req
	return f.response, f.err
}

func TestHandlerReturnsStableJSONContract(t *testing.T) {
	searcher := &fakeSearcher{response: searchsvc.Response{
		Query: "test", Normalized: "test", UsedQuery: "test", Total: 1, TookMS: 4,
		Results: []searchsvc.Result{{ID: 1, Title: "Result", URL: "https://example.com", Host: "example.com", Snippet: "snippet"}},
	}}
	h := Handler{SearchService: searcher}
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=5", nil)
	rr := httptest.NewRecorder()
	h.Search(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	if searcher.request.Query != "test" || searcher.request.Limit != 5 { t.Fatalf("request=%+v", searcher.request) }
	var payload searchsvc.Response
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil { t.Fatal(err) }
	if payload.Total != 1 || len(payload.Results) != 1 { t.Fatalf("payload=%+v", payload) }
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" { t.Fatal("missing nosniff") }
}

func TestHandlerRejectsInvalidLimit(t *testing.T) {
	h := Handler{SearchService: &fakeSearcher{}}
	for _, path := range []string{"/api/search?q=x&limit=0", "/api/search?q=x&limit=21", "/api/search?q=x&limit=nope"} {
		rr := httptest.NewRecorder()
		h.Search(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusBadRequest { t.Fatalf("path=%s status=%d", path, rr.Code) }
	}
}

func TestHandlerMapsQueryBackendAndDeadlineErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{querynorm.ErrEmptyQuery, http.StatusBadRequest},
		{querynorm.ErrQueryTooLong, http.StatusBadRequest},
		{querynorm.ErrQueryTooComplex, http.StatusBadRequest},
		{context.DeadlineExceeded, http.StatusGatewayTimeout},
		{errors.New("backend down"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		h := Handler{SearchService: &fakeSearcher{err: tc.err}}
		rr := httptest.NewRecorder()
		h.Search(rr, httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil))
		if rr.Code != tc.want { t.Fatalf("err=%v status=%d want=%d", tc.err, rr.Code, tc.want) }
	}
}
