package trap

import (
	"errors"
	"strings"
	"testing"
)

func TestGuardAllowsNormalURL(t *testing.T) {
	if err := Check("https://example.com/catalog/item?id=10&sort=price", DefaultLimits()); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}

func TestGuardRejectsKnownTrapPatterns(t *testing.T) {
	tests := []struct {
		url  string
		want error
	}{
		{"https://example.com/?PHPSESSID=abc", ErrSessionParameter},
		{"https://example.com/search?q=iphone", ErrSearchPage},
		{"https://example.com/a/a/a", ErrRepeatedSegments},
	}
	for _, tt := range tests {
		if err := Check(tt.url, DefaultLimits()); !errors.Is(err, tt.want) {
			t.Fatalf("Check(%q) error=%v want=%v", tt.url, err, tt.want)
		}
	}
}

func TestGuardHardLimits(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxURLLength = 20
	if err := Check("https://example.com/very-long-path", limits); !errors.Is(err, ErrURLTooLong) {
		t.Fatalf("expected URL length error, got %v", err)
	}

	limits = DefaultLimits()
	limits.MaxPathDepth = 2
	if err := Check("https://example.com/a/b/c", limits); !errors.Is(err, ErrPathTooDeep) {
		t.Fatalf("expected path depth error, got %v", err)
	}

	limits = DefaultLimits()
	limits.MaxQueryParams = 2
	if err := Check("https://example.com/?a=1&b=2&c=3", limits); !errors.Is(err, ErrTooManyParams) {
		t.Fatalf("expected query param error, got %v", err)
	}

	limits = DefaultLimits()
	tooLong := "https://example.com/" + strings.Repeat("a", limits.MaxURLLength)
	if err := Check(tooLong, limits); !errors.Is(err, ErrURLTooLong) {
		t.Fatalf("expected hard URL length error, got %v", err)
	}
}
