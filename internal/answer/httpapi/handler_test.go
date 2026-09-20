package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	answersvc "github.com/venomimonstro/poisk/internal/answer"
	querynorm "github.com/venomimonstro/poisk/internal/query"
)

type fakeAnswerer struct {
	response answersvc.Response
	err      error
	request  answersvc.Request
}

func (f *fakeAnswerer) Answer(_ context.Context, req answersvc.Request) (answersvc.Response, error) {
	f.request = req
	return f.response, f.err
}

func TestHandlerReturnsAnswerJSON(t *testing.T) {
	answerer := &fakeAnswerer{response: answersvc.Response{
		Available: true,
		Answer: "Факт из источника",
		Claims: []answersvc.Claim{{Text: "Факт из источника", SourceIDs: []int{1}}},
		Sources: []answersvc.Source{{ID: 1, URL: "https://example.com", Host: "example.com"}},
		Confidence: 0.8,
	}}
	h := Handler{AnswerService: answerer}
	rr := httptest.NewRecorder()
	h.Answer(rr, httptest.NewRequest(http.MethodGet, "/api/answer?q=test", nil))
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	if answerer.request.Query != "test" { t.Fatalf("request=%+v", answerer.request) }
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" { t.Fatal("missing nosniff") }
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
		h := Handler{AnswerService: &fakeAnswerer{err: tc.err}}
		rr := httptest.NewRecorder()
		h.Answer(rr, httptest.NewRequest(http.MethodGet, "/api/answer?q=x", nil))
		if rr.Code != tc.want { t.Fatalf("err=%v status=%d want=%d", tc.err, rr.Code, tc.want) }
	}
}
