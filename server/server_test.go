package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bug-bounty-engine/ai"
	"bug-bounty-engine/api"
	"bug-bounty-engine/engine"
	"bug-bounty-engine/model"
)

func TestExplainEndpointReturnsMultiAgentFields(t *testing.T) {
	fetchService := api.NewService(
		time.Minute,
		time.Now,
		testProvider{bugs: []model.Bug{
			{
				ID:                101,
				Title:             "Auth bypass",
				Severity:          5,
				Age:               10,
				BountyValue:       800,
				Reproductions:     3,
				EstimatedFixHours: 4,
				Source:            "github",
				URL:               "https://github.com/OWASP-BLT/BLT-Pages/issues/101",
			},
		}},
	)
	core := engine.New(fetchService)
	srv := New(core, "", stubExplainer{})

	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + "/api/explain/101")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if payload["summary"] == nil || payload["detail"] == nil {
		t.Fatalf("missing summary/detail in payload: %+v", payload)
	}
	agents, ok := payload["agents"].(map[string]any)
	if !ok {
		t.Fatalf("missing agents object in payload: %+v", payload)
	}
	if agents["security"] == nil || agents["optimization"] == nil {
		t.Fatalf("missing security/optimization agent fields: %+v", agents)
	}
}

type testProvider struct {
	bugs []model.Bug
}

func (p testProvider) Name() string { return "test" }

func (p testProvider) Fetch(context.Context) ([]model.Bug, error) {
	return p.bugs, nil
}

type stubExplainer struct{}

func (stubExplainer) Explain(context.Context, model.Bug) ai.ExplainResult {
	return ai.ExplainResult{
		Summary: "stub summary",
		Detail:  "stub detail",
		Security: ai.AgentExplanation{
			Name:     "Sentinel",
			Focus:    "security",
			Analysis: "Risk: high",
		},
		Optimization: ai.AgentExplanation{
			Name:     "Optimizer",
			Focus:    "optimization",
			Analysis: "Plan: optimize",
		},
		Provider: "stub",
		Model:    "stub-model",
		Fallback: false,
	}
}
