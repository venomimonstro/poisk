package scheduler

import "testing"

func TestComputePriority(t *testing.T) {
	got, err := ComputePriority(10, 0.8, 0.5, 0.25, 2)
	if err != nil {
		t.Fatalf("ComputePriority() error = %v", err)
	}
	if got <= 0 {
		t.Fatalf("expected positive priority, got %v", got)
	}
}

func TestComputePriorityRejectsInvalidInput(t *testing.T) {
	for _, tc := range [][5]float64{
		{-1, 1, 1, 1, 1},
		{1, -1, 1, 1, 1},
		{1, 1, 1, 1, 0},
	} {
		if _, err := ComputePriority(tc[0], tc[1], tc[2], tc[3], tc[4]); err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}

func TestComputePriorityHigherDemandWins(t *testing.T) {
	low, _ := ComputePriority(1, 1, 1, 1, 1)
	high, _ := ComputePriority(10, 1, 1, 1, 1)
	if high <= low {
		t.Fatalf("higher demand should increase priority: low=%v high=%v", low, high)
	}
}
