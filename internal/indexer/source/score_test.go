package source

import (
	"math"
	"testing"
)

func TestValidScore(t *testing.T) {
	for _, value := range []float64{0, 1, 50.5, 100} {
		if !validScore(value) { t.Fatalf("expected valid score %v", value) }
	}
	for _, value := range []float64{-1, 100.01, math.Inf(1), math.Inf(-1), math.NaN()} {
		if validScore(value) { t.Fatalf("expected invalid score %v", value) }
	}
}
