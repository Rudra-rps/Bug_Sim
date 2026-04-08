package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"bug-bounty-engine/engine"
	"bug-bounty-engine/model"
	"bug-bounty-engine/scheduler"
)

type HTTPServer struct {
	engine      *engine.Engine
	frontendDir string
}

func New(engine *engine.Engine, frontendDir string) *HTTPServer {
	return &HTTPServer{
		engine:      engine,
		frontendDir: frontendDir,
	}
}

func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/bugs", s.handleGetBugs)
	mux.HandleFunc("GET /api/top", s.handleGetTop)
	mux.HandleFunc("POST /api/fix/{id}", s.handleFixBug)
	mux.HandleFunc("GET /api/explain/{id}", s.handleExplainBug)
	mux.HandleFunc("GET /api/schedule", s.handleSchedule)
	mux.HandleFunc("GET /api/compare", s.handleCompare)

	if s.frontendDir != "" {
		if stat, err := os.Stat(s.frontendDir); err == nil && stat.IsDir() {
			mux.Handle("/", http.FileServer(http.Dir(s.frontendDir)))
			return mux
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Bug bounty engine backend is running."))
	})

	return mux
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleGetBugs(w http.ResponseWriter, r *http.Request) {
	bugs, err := s.engine.All(r.Context(), parseRefresh(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, bugs)
}

func (s *HTTPServer) handleGetTop(w http.ResponseWriter, r *http.Request) {
	k := parseIntOrDefault(r.URL.Query().Get("k"), 5)
	if k <= 0 {
		k = 5
	}

	bugs, err := s.engine.TopK(r.Context(), k, parseRefresh(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"k":    k,
		"bugs": bugs,
	})
}

func (s *HTTPServer) handleFixBug(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid bug id"))
		return
	}

	bug, removed, err := s.engine.FixByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if !removed {
		writeError(w, http.StatusNotFound, errors.New("bug not found"))
		return
	}

	top, err := s.engine.TopK(r.Context(), 8, false)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"fixed": bug,
		"top":   top,
	})
}

func (s *HTTPServer) handleSchedule(w http.ResponseWriter, r *http.Request) {
	hours := parseIntOrDefault(r.URL.Query().Get("hours"), 8)
	if hours < 0 {
		writeError(w, http.StatusBadRequest, errors.New("hours must be positive"))
		return
	}

	result, err := s.engine.Schedule(r.Context(), hours)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result.Bugs)
}

func (s *HTTPServer) handleExplainBug(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid bug id"))
		return
	}

	bug, ok, err := s.engine.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("bug not found"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bug":     bug,
		"summary": explainSummary(bug),
		"detail":  explainDetail(bug),
	})
}

func (s *HTTPServer) handleCompare(w http.ResponseWriter, r *http.Request) {
	hours := parseIntOrDefault(r.URL.Query().Get("hours"), 8)
	if hours < 0 {
		writeError(w, http.StatusBadRequest, errors.New("hours must be positive"))
		return
	}

	cap := parseIntOrDefault(r.URL.Query().Get("cap"), scheduler.DefaultBruteForceCandidateCap)
	if cap <= 0 {
		cap = scheduler.DefaultBruteForceCandidateCap
	}

	result, err := s.engine.Compare(r.Context(), hours, cap)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	greedyCount := len(result.Greedy.Bugs)
	optimalCount := len(result.BruteForce.Bugs)
	writeJSON(w, http.StatusOK, map[string]any{
		"greedy": map[string]any{
			"totalPriority": result.Greedy.TotalPriority,
			"count":         greedyCount,
			"hoursUsed":     result.Greedy.UsedHours,
		},
		"optimal": map[string]any{
			"totalPriority": result.BruteForce.TotalPriority,
			"count":         optimalCount,
			"hoursUsed":     result.BruteForce.UsedHours,
		},
		"details": result,
	})
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseIntOrDefault(raw string, defaultValue int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

func parseRefresh(r *http.Request) bool {
	value := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("refresh")))
	return value == "1" || value == "true" || value == "yes"
}

func explainSummary(bug model.Bug) string {
	return "This bug is prioritized because its combined severity, bounty, reproducibility, and age produce a high weighted score."
}

func explainDetail(bug model.Bug) string {
	components := []string{
		"Severity contributes " + scoreComponentLabel(bug.PriorityBreakdown.Severity),
		"Bounty contributes " + scoreComponentLabel(bug.PriorityBreakdown.BountyValue),
		"Reproductions contribute " + scoreComponentLabel(bug.PriorityBreakdown.Reproductions),
		"Age contributes " + scoreComponentLabel(bug.PriorityBreakdown.Age),
	}
	return strings.Join(components, ". ") + "."
}

func scoreComponentLabel(value float64) string {
	switch {
	case value >= 80:
		return "very strongly"
	case value >= 60:
		return "strongly"
	case value >= 40:
		return "moderately"
	case value >= 20:
		return "lightly"
	default:
		return "minimally"
	}
}
