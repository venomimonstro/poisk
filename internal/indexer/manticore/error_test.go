package manticore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRawSQLErrorIsRejectedEvenWithHTTP200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"data":[],"total":0,"error":"table missing","warning":""}]`)
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL})
	if err != nil { t.Fatal(err) }
	_, _, err = c.CurrentVersion(context.Background(), 1)
	if !errors.Is(err, ErrRemote) { t.Fatalf("err=%v", err) }
}
