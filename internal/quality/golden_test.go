package quality

import (
	"strings"
	"testing"
)

func TestLoadGoldenValidatesSchemaAndURLs(t *testing.T) {
	input := `{"version":1,"queries":[{"id":"q1","query":"пример","tags":["web"],"judgments":[{"url":"HTTPS://Example.com/path#fragment","grade":3}]}]}`
	set, err := LoadGolden(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if set.Version != 1 || len(set.Queries) != 1 {
		t.Fatalf("set=%+v", set)
	}
}

func TestGoldenRejectsDuplicateIDs(t *testing.T) {
	input := `{"version":1,"queries":[{"id":"same","query":"a","judgments":[]},{"id":"same","query":"b","judgments":[]}]}`
	if _, err := LoadGolden(strings.NewReader(input)); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestGoldenRejectsUnknownFields(t *testing.T) {
	input := `{"version":1,"queries":[{"id":"q1","query":"a","judgments":[],"unexpected":true}]}`
	if _, err := LoadGolden(strings.NewReader(input)); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestGoldenRejectsInvalidGrade(t *testing.T) {
	input := `{"version":1,"queries":[{"id":"q1","query":"a","judgments":[{"url":"https://example.com/","grade":4}]}]}`
	if _, err := LoadGolden(strings.NewReader(input)); err == nil {
		t.Fatal("expected grade error")
	}
}
