package scorer

import "bug-bounty-engine/model"

const (
	severityWeight      = 0.4
	bountyValueWeight   = 0.3
	reproductionsWeight = 0.2
	ageWeight           = 0.1
)

type metricRange struct {
	min float64
	max float64
}

// ScoreStats captures the input ranges required to normalize a bug's fields to
// the 0-100 scale expected by the weighted priority formula.
type ScoreStats struct {
	severity      metricRange
	bountyValue   metricRange
	reproductions metricRange
	age           metricRange
}

// BuildStats computes normalization ranges from the current bug set.
func BuildStats(bugs []model.Bug) ScoreStats {
	stats := ScoreStats{}
	if len(bugs) == 0 {
		return stats
	}

	stats.severity = metricRange{min: float64(bugs[0].Severity), max: float64(bugs[0].Severity)}
	stats.bountyValue = metricRange{min: float64(bugs[0].BountyValue), max: float64(bugs[0].BountyValue)}
	stats.reproductions = metricRange{min: float64(bugs[0].Reproductions), max: float64(bugs[0].Reproductions)}
	stats.age = metricRange{min: float64(bugs[0].Age), max: float64(bugs[0].Age)}

	for _, bug := range bugs[1:] {
		stats.severity = expandRange(stats.severity, float64(bug.Severity))
		stats.bountyValue = expandRange(stats.bountyValue, float64(bug.BountyValue))
		stats.reproductions = expandRange(stats.reproductions, float64(bug.Reproductions))
		stats.age = expandRange(stats.age, float64(bug.Age))
	}

	return stats
}

// ScoreBug returns a copy of the bug with normalized breakdown fields and the
// final weighted priority score populated.
func ScoreBug(bug model.Bug, stats ScoreStats) model.Bug {
	breakdown := model.ScoreBreakdown{
		Severity:      normalize(float64(bug.Severity), stats.severity),
		BountyValue:   normalize(float64(bug.BountyValue), stats.bountyValue),
		Reproductions: normalize(float64(bug.Reproductions), stats.reproductions),
		Age:           normalize(float64(bug.Age), stats.age),
	}

	bug.PriorityBreakdown = breakdown
	bug.Priority = (breakdown.Severity * severityWeight) +
		(breakdown.BountyValue * bountyValueWeight) +
		(breakdown.Reproductions * reproductionsWeight) +
		(breakdown.Age * ageWeight)

	return bug
}

// ScoreBugs returns a new slice with priorities computed using the same
// normalization ranges across the full set.
func ScoreBugs(bugs []model.Bug) []model.Bug {
	stats := BuildStats(bugs)
	scored := make([]model.Bug, 0, len(bugs))

	for _, bug := range bugs {
		scored = append(scored, ScoreBug(bug, stats))
	}

	return scored
}

func expandRange(current metricRange, value float64) metricRange {
	if value < current.min {
		current.min = value
	}
	if value > current.max {
		current.max = value
	}
	return current
}

func normalize(value float64, r metricRange) float64 {
	if r.max == r.min {
		if value == 0 {
			return 0
		}
		return 100
	}
	return ((value - r.min) / (r.max - r.min)) * 100
}
