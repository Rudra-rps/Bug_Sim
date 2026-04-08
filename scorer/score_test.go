package scorer

import (
	"math"
	"testing"

	"bug-bounty-engine/model"
)

func TestScoreBugsOrdersHigherImpactBugFirst(t *testing.T) {
	bugs := []model.Bug{
		{
			ID:            1,
			Title:         "Low impact bug",
			Severity:      1,
			Age:           5,
			BountyValue:   50,
			Reproductions: 1,
		},
		{
			ID:            2,
			Title:         "High impact bug",
			Severity:      5,
			Age:           40,
			BountyValue:   700,
			Reproductions: 12,
		},
	}

	scored := ScoreBugs(bugs)

	if scored[1].Priority <= scored[0].Priority {
		t.Fatalf("expected bug 2 priority %.2f to exceed bug 1 priority %.2f", scored[1].Priority, scored[0].Priority)
	}

	if !almostEqual(scored[1].Priority, 100) {
		t.Fatalf("expected dominant bug to normalize to priority 100, got %.2f", scored[1].Priority)
	}

	if !almostEqual(scored[0].Priority, 0) {
		t.Fatalf("expected weaker bug to normalize to priority 0, got %.2f", scored[0].Priority)
	}
}

func TestScoreBugTracksBreakdown(t *testing.T) {
	stats := BuildStats([]model.Bug{
		{Severity: 1, Age: 10, BountyValue: 100, Reproductions: 1},
		{Severity: 5, Age: 30, BountyValue: 500, Reproductions: 5},
	})

	scored := ScoreBug(model.Bug{
		Severity:      3,
		Age:           20,
		BountyValue:   300,
		Reproductions: 3,
	}, stats)

	if !almostEqual(scored.PriorityBreakdown.Severity, 50) {
		t.Fatalf("expected severity breakdown 50, got %.2f", scored.PriorityBreakdown.Severity)
	}
	if !almostEqual(scored.PriorityBreakdown.BountyValue, 50) {
		t.Fatalf("expected bounty breakdown 50, got %.2f", scored.PriorityBreakdown.BountyValue)
	}
	if !almostEqual(scored.PriorityBreakdown.Reproductions, 50) {
		t.Fatalf("expected reproductions breakdown 50, got %.2f", scored.PriorityBreakdown.Reproductions)
	}
	if !almostEqual(scored.PriorityBreakdown.Age, 50) {
		t.Fatalf("expected age breakdown 50, got %.2f", scored.PriorityBreakdown.Age)
	}
	if !almostEqual(scored.Priority, 50) {
		t.Fatalf("expected overall priority 50, got %.2f", scored.Priority)
	}
}

func TestNormalizeAllSameNonZeroKeepsSignal(t *testing.T) {
	bugs := []model.Bug{
		{Severity: 3, Age: 10, BountyValue: 100, Reproductions: 1},
		{Severity: 3, Age: 10, BountyValue: 100, Reproductions: 1},
	}

	scored := ScoreBugs(bugs)

	for i, bug := range scored {
		if !almostEqual(bug.Priority, 100) {
			t.Fatalf("expected bug %d priority 100 with identical non-zero metrics, got %.2f", i, bug.Priority)
		}
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}
