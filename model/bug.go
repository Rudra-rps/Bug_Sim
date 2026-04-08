package model

// Bug is the shared domain object used across scoring, heap operations, and
// later scheduling/API layers.
type Bug struct {
	ID                int            `json:"id"`
	Title             string         `json:"title"`
	Severity          int            `json:"severity"`
	Age               int            `json:"age"`
	BountyValue       int            `json:"bountyValue"`
	Reproductions     int            `json:"reproductions"`
	EstimatedFixHours int            `json:"estimatedFixHours"`
	Source            string         `json:"source"`
	URL               string         `json:"url"`
	Priority          float64        `json:"priority"`
	PriorityBreakdown ScoreBreakdown `json:"priorityBreakdown"`
}

// ScoreBreakdown keeps normalized component scores so the frontend and API can
// explain why a bug was prioritized.
type ScoreBreakdown struct {
	Severity      float64 `json:"severity"`
	BountyValue   float64 `json:"bountyValue"`
	Reproductions float64 `json:"reproductions"`
	Age           float64 `json:"age"`
}
