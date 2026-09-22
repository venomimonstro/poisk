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

func TestSearchPayloadHasWeightsHighlightAndSignals(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" { t.Fatalf("path=%s", r.URL.Path) }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil { t.Fatal(err) }
		_, _ = io.WriteString(w, "{\"took\":3,\"timed_out\":false,\"hits\":{\"total\":1,\"hits\":[{\"_id\":1,\"_score\":12.5,\"_source\":{\"title\":\"Title\",\"description\":\"Description\",\"url\":\"https://example.com/a\",\"host\":\"example.com\",\"lang\":\"ru\",\"quality_score\":88,\"spam_score\":7,\"authority_score\":42,\"fetched_at\":1789999200},\"highlight\":{\"body\":[\"text [[match]] tail\"]}}]}}")
	}))
	defer srv.Close()

	client, err := New(Config{BaseURL: srv.URL, MaxResults: 20})
	if err != nil { t.Fatal(err) }
	result, err := client.Search(context.Background(), "поиск", 100)
	if err != nil { t.Fatal(err) }
	if len(result.Hits) != 1 || result.Hits[0].Snippet != "text [[match]] tail" { t.Fatalf("result=%+v", result) }
	if result.Hits[0].QualityScore != 88 || result.Hits[0].SpamScore != 7 || result.Hits[0].AuthorityScore != 42 || result.Hits[0].FetchedAtUnix != 1789999200 {
		t.Fatalf("signals=%+v", result.Hits[0])
	}
	if got := int(payload["limit"].(float64)); got != 20 { t.Fatalf("limit=%d", got) }
	source := payload["_source"].([]any)
	foundAuthority := false;foundFetched:=false
	for _, value := range source { if value == "authority_score" { foundAuthority = true };if value=="fetched_at"{foundFetched=true} }
	if !foundAuthority || !foundFetched { t.Fatalf("source=%v", source) }
	highlight := payload["highlight"].(map[string]any)
	if highlight["before_match"] != "[[" || highlight["after_match"] != "]]" { t.Fatalf("highlight=%v", highlight) }
	options := payload["options"].(map[string]any)
	weights := options["field_weights"].(map[string]any)
	if int(weights["title"].(float64)) != 12 || int(weights["description"].(float64)) != 4 || int(weights["body"].(float64)) != 1 { t.Fatalf("weights=%v", weights) }
}

func TestSearchClampsOfflineSignals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"took\":1,\"timed_out\":false,\"hits\":{\"total\":1,\"hits\":[{\"_id\":1,\"_score\":1,\"_source\":{\"url\":\"https://example.com\",\"host\":\"example.com\",\"quality_score\":1000,\"spam_score\":-5,\"authority_score\":999}}]}}")
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL})
	result, err := client.Search(context.Background(), "x", 10)
	if err != nil { t.Fatal(err) }
	hit := result.Hits[0]
	if hit.QualityScore != 100 || hit.SpamScore != 0 || hit.AuthorityScore != 100 { t.Fatalf("hit=%+v", hit) }
}

func TestSearchRejectsTimedOutBackendResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"took\":1200,\"timed_out\":true,\"hits\":{\"total\":0,\"hits\":[]}}")
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL})
	_, err := client.Search(context.Background(), "x", 10)
	if !errors.Is(err, ErrSearchBackend) { t.Fatalf("err=%v", err) }
}

func TestSearchResponseHardLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = fmt.Fprint(w, strings.Repeat("x", 100))
	}))
	defer srv.Close()
	client, _ := New(Config{BaseURL: srv.URL, MaxResponseBytes: 8})
	_, err := client.Search(context.Background(), "x", 10)
	if !errors.Is(err, ErrResponseTooLarge) { t.Fatalf("err=%v", err) }
}
