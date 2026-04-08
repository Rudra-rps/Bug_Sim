package scheduler

import (
	"sort"

	"bug-bounty-engine/model"
)

const DefaultBruteForceCandidateCap = 16

type ScheduleResult struct {
	Bugs          []model.Bug `json:"bugs"`
	BudgetHours   int         `json:"budgetHours"`
	UsedHours     int         `json:"usedHours"`
	UnusedHours   int         `json:"unusedHours"`
	TotalPriority float64     `json:"totalPriority"`
}

type CompareResult struct {
	Greedy              ScheduleResult `json:"greedy"`
	BruteForce          ScheduleResult `json:"bruteForce"`
	BruteForceCandidate int            `json:"bruteForceCandidateCount"`
	BruteForceCapped    bool           `json:"bruteForceCapped"`
	GreedyMatchesBest   bool           `json:"greedyMatchesBest"`
}

func Greedy(bugs []model.Bug, budgetHours int) ScheduleResult {
	if budgetHours <= 0 || len(bugs) == 0 {
		return ScheduleResult{BudgetHours: max(0, budgetHours)}
	}

	ordered := append([]model.Bug(nil), bugs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		iHours := normalizeHours(ordered[i].EstimatedFixHours)
		jHours := normalizeHours(ordered[j].EstimatedFixHours)
		iRatio := ordered[i].Priority / float64(iHours)
		jRatio := ordered[j].Priority / float64(jHours)
		if iRatio == jRatio {
			if ordered[i].Priority == ordered[j].Priority {
				return iHours < jHours
			}
			return ordered[i].Priority > ordered[j].Priority
		}
		return iRatio > jRatio
	})

	used := 0
	selected := make([]model.Bug, 0, len(ordered))
	totalPriority := 0.0
	for _, bug := range ordered {
		hours := normalizeHours(bug.EstimatedFixHours)
		if used+hours > budgetHours {
			continue
		}
		used += hours
		selected = append(selected, bug)
		totalPriority += bug.Priority
	}

	return ScheduleResult{
		Bugs:          selected,
		BudgetHours:   budgetHours,
		UsedHours:     used,
		UnusedHours:   budgetHours - used,
		TotalPriority: totalPriority,
	}
}

func BruteForce(bugs []model.Bug, budgetHours int, candidateCap int) ScheduleResult {
	result, _, _ := bruteForceInternal(bugs, budgetHours, candidateCap)
	return result
}

func Compare(bugs []model.Bug, budgetHours int, candidateCap int) CompareResult {
	greedy := Greedy(bugs, budgetHours)
	brute, candidateCount, capped := bruteForceInternal(bugs, budgetHours, candidateCap)

	return CompareResult{
		Greedy:              greedy,
		BruteForce:          brute,
		BruteForceCandidate: candidateCount,
		BruteForceCapped:    capped,
		GreedyMatchesBest:   almostEqual(greedy.TotalPriority, brute.TotalPriority),
	}
}

func bruteForceInternal(bugs []model.Bug, budgetHours int, candidateCap int) (ScheduleResult, int, bool) {
	if budgetHours <= 0 || len(bugs) == 0 {
		return ScheduleResult{BudgetHours: max(0, budgetHours)}, 0, false
	}

	if candidateCap <= 0 {
		candidateCap = DefaultBruteForceCandidateCap
	}

	candidates := append([]model.Bug(nil), bugs...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	capped := false
	if len(candidates) > candidateCap {
		candidates = candidates[:candidateCap]
		capped = true
	}

	n := len(candidates)
	bestPriority := -1.0
	bestHours := 0
	bestMask := 0

	limit := 1 << n
	for mask := 0; mask < limit; mask++ {
		used := 0
		total := 0.0
		valid := true
		for i := 0; i < n; i++ {
			if mask&(1<<i) == 0 {
				continue
			}
			hours := normalizeHours(candidates[i].EstimatedFixHours)
			used += hours
			if used > budgetHours {
				valid = false
				break
			}
			total += candidates[i].Priority
		}

		if !valid {
			continue
		}
		if total > bestPriority || (almostEqual(total, bestPriority) && used < bestHours) {
			bestPriority = total
			bestHours = used
			bestMask = mask
		}
	}

	selected := make([]model.Bug, 0, n)
	for i := 0; i < n; i++ {
		if bestMask&(1<<i) != 0 {
			selected = append(selected, candidates[i])
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Priority > selected[j].Priority
	})

	if bestPriority < 0 {
		bestPriority = 0
	}

	return ScheduleResult{
		Bugs:          selected,
		BudgetHours:   budgetHours,
		UsedHours:     bestHours,
		UnusedHours:   budgetHours - bestHours,
		TotalPriority: bestPriority,
	}, n, capped
}

func normalizeHours(hours int) int {
	if hours <= 0 {
		return 1
	}
	return hours
}

func almostEqual(a, b float64) bool {
	const epsilon = 0.0001
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}
