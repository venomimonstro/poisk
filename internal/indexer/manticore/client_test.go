package manticore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQuoteEscapesSQLLiteral(t *testing.T) {
	got := quote("a'b\\c\n")
	want := "'a\\'b\\\\c\\n'"
	if got != want { t.Fatalf("quote=%q want=%q", got, want) }
}

func TestCountDocumentsParsesBoundedResult(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		query = string(body)
		_, _ = io.WriteString(w, "[{\"data\":[{\"count\":123}],\"total\":1,\"error\":\"\",\"warning\":\"\"}]")
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL})
	if err != nil { t.Fatal(err) }
	count, err := c.CountDocuments(context.Background())
	if err != nil { t.Fatal(err) }
	if count != 123 { t.Fatalf("count=%d", count) }
	if query != "SELECT COUNT(*) AS count FROM web_documents" { t.Fatalf("query=%q", query) }
}

func TestCurrentVersionParsesRawResultSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "[{\"columns\":[{\"entity_version\":{\"type\":\"long long\"}}],\"data\":[{\"entity_version\":7}],\"total\":1,\"error\":\"\",\"warning\":\"\"}]")
	}))
	defer srv.Close()
	c, err := New(Config{BaseURL: srv.URL})
	if err != nil { t.Fatal(err) }
	v, exists, err := c.CurrentVersion(context.Background(), 10)
	if err != nil { t.Fatal(err) }
	if !exists || v != 7 { t.Fatalf("version=%d exists=%v", v, exists) }
}

func TestApplySkipsStaleVersion(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if calls == 1 {
			if !strings.Contains(string(body), "SELECT entity_version") { t.Fatalf("query=%s", body) }
			_, _ = io.WriteString(w, "[{\"data\":[{\"entity_version\":9}],\"total\":1,\"error\":\"\",\"warning\":\"\"}]")
			return
		}
		t.Fatalf("stale document triggered mutation: %s", body)
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL})
	applied, err := c.Apply(context.Background(), Document{ID: 1, EntityVersion: 8})
	if err != nil { t.Fatal(err) }
	if applied { t.Fatal("stale document must be skipped") }
	if calls != 1 { t.Fatalf("calls=%d", calls) }
}

func TestApplyReplacesEqualOrNewerVersion(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		queries = append(queries, string(body))
		if len(queries) == 1 {
			_, _ = io.WriteString(w, "[{\"data\":[{\"entity_version\":\"3\"}],\"total\":1,\"error\":\"\",\"warning\":\"\"}]")
			return
		}
		_, _ = io.WriteString(w, "[{\"total\":0,\"error\":\"\",\"warning\":\"\"}]")
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL})
	ok, err := c.Apply(context.Background(), Document{ID: 5, EntityVersion: 4, Title: "O'Reilly", Body: "body", AuthorityScore: 25})
	if err != nil { t.Fatal(err) }
	if !ok { t.Fatal("expected apply") }
	if len(queries) != 2 || !strings.Contains(queries[1], "REPLACE INTO web_documents") || !strings.Contains(queries[1], "O\\'Reilly") || !strings.Contains(queries[1], "authority_score") {
		t.Fatalf("queries=%v", queries)
	}
}

func TestEnsureSchemaAddsAuthorityOnlyWhenMissing(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		queries = append(queries, q)
		switch {
		case strings.HasPrefix(q, "CREATE TABLE"):
			_, _ = io.WriteString(w, "[{\"total\":0,\"error\":\"\",\"warning\":\"\"}]")
		case strings.HasPrefix(q, "DESC "):
			_, _ = io.WriteString(w, "[{\"data\":[{\"Field\":\"id\",\"Type\":\"bigint\"},{\"Field\":\"quality_score\",\"Type\":\"float\"}],\"error\":\"\",\"warning\":\"\"}]")
		case strings.HasPrefix(q, "ALTER TABLE"):
			_, _ = io.WriteString(w, "[{\"total\":0,\"error\":\"\",\"warning\":\"\"}]")
		default:
			t.Fatalf("unexpected query %q", q)
		}
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL})
	if err := c.EnsureSchema(context.Background()); err != nil { t.Fatal(err) }
	if len(queries) != 3 || !strings.Contains(queries[2], "ADD COLUMN authority_score FLOAT") {
		t.Fatalf("queries=%v", queries)
	}
}

func TestRawHasColumnIsCaseInsensitive(t *testing.T) {
	body := []byte("[{\"data\":[{\"Field\":\"AUTHORITY_SCORE\"}],\"error\":\"\"}]")
	has, err := rawHasColumn(body, "authority_score")
	if err != nil { t.Fatal(err) }
	if !has { t.Fatal("expected authority column") }
}

func TestDeleteIsDeterministic(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body); query = string(b)
		_, _ = io.WriteString(w, "[{\"total\":0,\"error\":\"\",\"warning\":\"\"}]")
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL})
	if err := c.Delete(context.Background(), 42); err != nil { t.Fatal(err) }
	if query != "DELETE FROM web_documents WHERE id=42" { t.Fatalf("query=%q", query) }
}

func TestResponseHardLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = fmt.Fprint(w, strings.Repeat("x", 100))
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, MaxResponseBytes: 8})
	_, _, err := c.CurrentVersion(context.Background(), 1)
	if !errors.Is(err, ErrResponseTooLarge) { t.Fatalf("err=%v", err) }
}

func TestRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(w, "[]")
	}))
	defer srv.Close()
	c, _ := New(Config{BaseURL: srv.URL, RequestTimeout: 5 * time.Millisecond})
	_, _, err := c.CurrentVersion(context.Background(), 1)
	if err == nil { t.Fatal("expected timeout") }
}
