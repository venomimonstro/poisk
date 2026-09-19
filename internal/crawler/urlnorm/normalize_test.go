package urlnorm

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase host and default path", "HTTPS://Example.COM", "https://example.com/"},
		{"strip default port", "http://Example.com:80/a", "http://example.com/a"},
		{"preserve custom port", "https://Example.com:8443/a", "https://example.com:8443/a"},
		{"strip fragment", "https://example.com/a#part", "https://example.com/a"},
		{"remove trackers", "https://example.com/a?utm_source=x&gclid=1&id=5", "https://example.com/a?id=5"},
		{"sort query", "https://example.com/a?z=2&a=1", "https://example.com/a?a=1&z=2"},
		{"preserve meaningful ref", "https://example.com/a?ref=partner&id=5", "https://example.com/a?id=5&ref=partner"},
		{"idna", "https://пример.рф/путь", "https://xn--e1afmkfd.xn--p1ai/%D0%BF%D1%83%D1%82%D1%8C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeRejectsUnsafeForms(t *testing.T) {
	for _, raw := range []string{
		"ftp://example.com/file",
		"https://user:pass@example.com/",
		"https:///missing-host",
	} {
		if _, err := Normalize(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}
