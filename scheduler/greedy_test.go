package scheduler

import (
	"testing"

	"bug-bounty-engine/model"
)

func TestGreedyRespectsBudgetAndPicksBestRatio(t *testing.T) {
	bugs := []model.Bug{
		{ID: 1, Priority: 90, EstimatedFixHours: 6},
		{ID: 2, Priority: 70, EstimatedFixHours: 2},
		{ID: 3, Priority: 40, EstimatedFixHours: 1},
	}

	result := Greedy(bugs, 3)
	if result.UsedHours != 3 {
		t.Fatalf("expected used hours 3, got %d", result.UsedHours)
	}
	if len(result.Bugs) != 2 {
		t.Fatalf("expected 2 selected bugs, got %d", len(result.Bugs))
	}
	if result.Bugs[0].ID != 3 || result.Bugs[1].ID != 2 {
		t.Fatalf("expected bug ids [3,2], got [%d,%d]", result.Bugs[0].ID, result.Bugs[1].ID)
	}
}

func TestCompareIncludesBruteforceAndCapFlag(t *testing.T) {
	bugs := []model.Bug{
		{ID: 1, Priority: 100, EstimatedFixHours: 4},
		{ID: 2, Priority: 80, EstimatedFixHours: 4},
		{ID: 3, Priority: 30, EstimatedFixHours: 1},
		{ID: 4, Priority: 20, EstimatedFixHours: 1},
	}

	result := Compare(bugs, 5, 3)
	if !result.BruteForceCapped {
		t.Fatal("expected brute-force candidate cap to be marked as capped")
	}
	if result.BruteForceCandidate != 3 {
		t.Fatalf("expected candidate count 3, got %d", result.BruteForceCandidate)
	}
	if result.BruteForce.UsedHours > 5 {
		t.Fatalf("brute-force exceeded budget, used %d", result.BruteForce.UsedHours)
	}
}
