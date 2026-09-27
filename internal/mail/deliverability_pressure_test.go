package mail

import (
	"testing"
	"time"
)

func TestDeliveryPressureCooldownIsBounded(t *testing.T) {
	cases := []struct {
		failures int
		want time.Duration
	}{
		{0, 0}, {2, 0}, {3, 5 * time.Minute}, {5, 5 * time.Minute},
		{6, 15 * time.Minute}, {9, 15 * time.Minute}, {10, 30 * time.Minute}, {1000, 30 * time.Minute},
	}
	for _, tc := range cases {
		if got := deliveryPressureCooldown(tc.failures); got != tc.want {
			t.Fatalf("failures=%d cooldown=%s want=%s", tc.failures, got, tc.want)
		}
	}
}

func TestExternalRecipientDomainNormalizesDomain(t *testing.T) {
	got, err := externalRecipientDomain("User@Example.NET")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.net" {
		t.Fatalf("domain=%q", got)
	}
}
