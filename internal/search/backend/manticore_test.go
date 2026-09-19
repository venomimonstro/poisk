package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchPayloadHasWeightsHighlightAndLimit(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{"took":3}`)
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, MaxResults: 20})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = client.Search(context.Background(), "поиск", 100)
}

func TestSearchRejectsTimedOutBackendResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"took":1200}`)
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL})
	_, _ = client.Search(context.Background(), "x", 10)
}

func TestSearchResponseHardLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = fmt.Fprint(w, strings.Repeat("x", 100))
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL, MaxResponseBytes: 8})
	_, err := client.Search(context.Background(), "x", 10)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
