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
		_, _ = io.WriteString(w, "{\"took\":3,\"timed_out\":false,\"hits\":{\"total\":1,\"hits\":[{\"_id\":1,\"_score\":12.5,\"_source\":{\"title\":\"Title\",\"description\":\"Description\",\"url\":\"https://example.com/a\",\"host\":\"example.com\",\"lang\":\"ru\"},\"highlight\":{\"body\":[\"text [[match]] tail\"]}}]}}")
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, MaxResults: 20})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Search(context.Background(), "поиск", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].Snippet != "text [[match]] tail" {
		t.Fatalf("result=%+v", result)
	}
	if got := int(payload["limit"].(float64)); got != 20 {
		t.Fatalf("limit=%d", got)
	}
	highlight := payload["highlight"].(map[string]any)
	if highlight["before_match"] != "[[" || highlight["after_match"] != "]]" {
		t.Fatalf("highlight=%v", highlight)
	}
	options := payload["options"].(map[string]any)
	weights := options["field_weights"].(map[string]any)
	if int(weights["title"].(float64)) != 12 || int(weights["description"].(float64)) != 4 || int(weights["body"].(float64)) != 1 {
		t.Fatalf("weights=%v", weights)
	}
}

func TestSearchRejectsTimedOutBackendResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"took\":1200,\"timed_out\":true,\"hits\":{\"total\":0,\"hits\":[]}}")
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL})
	_, err := client.Search(context.Background(), "x", 10)
	if !errors.Is(err, ErrSearchBackend) {
		t.Fatalf("err=%v", err)
	}
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
