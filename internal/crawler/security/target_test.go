package security

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type fakeResolver map[string][]netip.Addr

func (f fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	ips, ok := f[host]
	if !ok {
		return nil, errors.New("not found")
	}
	return ips, nil
}

func TestValidatorAllowsPublicTargets(t *testing.T) {
	v := NewValidator()
	v.Resolver = fakeResolver{
		"example.com": {netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("2606:4700:4700::1111")},
	}
	got, err := v.Validate(context.Background(), "https://example.com/path")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got.Port != 443 || len(got.IPs) != 2 {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestValidatorRejectsPrivateOrMetadataTargets(t *testing.T) {
	v := NewValidator()
	v.Resolver = fakeResolver{
		"private.test":  {netip.MustParseAddr("10.0.0.1")},
		"metadata.test": {netip.MustParseAddr("169.254.169.254")},
		"mixed.test":    {netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")},
	}
	for _, raw := range []string{
		"http://private.test/",
		"http://metadata.test/",
		"https://mixed.test/",
		"http://127.0.0.1/",
		"http://[::1]/",
		"http://localhost/",
	} {
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}

func TestValidatorRejectsUnsupportedPortAndUserInfo(t *testing.T) {
	v := NewValidator()
	v.Resolver = fakeResolver{"example.com": {netip.MustParseAddr("1.1.1.1")}}
	for _, raw := range []string{
		"https://example.com:8443/",
		"https://user:pass@example.com/",
		"ftp://example.com/",
	} {
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}
