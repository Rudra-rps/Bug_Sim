package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"bug-bounty-engine/model"
)

func TestExplainFallsBackWithoutAPIKey(t *testing.T) {
	explainer := &GroqExplainer{
		client:      &http.Client{Timeout: 2 * time.Second},
		apiURL:      "https://example.invalid",
		apiKey:      "",
		model:       "test-model",
		timeout:     2 * time.Second,
		temperature: 0.2,
		maxTokens:   120,
	}

	result := explainer.Explain(context.Background(), sampleBug())
	if !result.Fallback {
		t.Fatalf("expected fallback mode when API key is missing")
	}
	if result.Security.Analysis == "" || result.Optimization.Analysis == "" {
		t.Fatalf("expected deterministic agent analysis in fallback mode")
	}
	if !strings.Contains(result.Reason, "GROQ_API_KEY") {
		t.Fatalf("expected fallback reason to mention missing key, got %q", result.Reason)
	}
}

func TestExplainCallsBothPersonas(t *testing.T) {
	var (
		mu                sync.Mutex
		securityCalls     int
		optimizationCalls int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req groqChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Messages) == 0 {
			t.Fatalf("expected chat messages")
		}

		system := strings.ToLower(req.Messages[0].Content)
		out := "generic"

		mu.Lock()
		switch {
		case strings.Contains(system, "security engineer"):
			securityCalls++
			out = "Risk: high\nExploitability: medium\nNext Fix: sanitize input"
		case strings.Contains(system, "efficiency strategist"):
			optimizationCalls++
			out = "Impact: high\nEffort: medium\nPlan: patch first then test"
		default:
			t.Fatalf("unexpected system prompt: %q", req.Messages[0].Content)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": out,
					},
				},
			},
		})
	}))
	defer server.Close()

	explainer := &GroqExplainer{
		client:      server.Client(),
		apiURL:      server.URL,
		apiKey:      "test-key",
		model:       "test-model",
		timeout:     3 * time.Second,
		temperature: 0.2,
		maxTokens:   120,
	}

	result := explainer.Explain(context.Background(), sampleBug())
	if result.Fallback {
		t.Fatalf("expected AI mode, got fallback with reason %q", result.Reason)
	}
	if !strings.Contains(result.Security.Analysis, "Risk:") {
		t.Fatalf("expected security output, got %q", result.Security.Analysis)
	}
	if !strings.Contains(result.Optimization.Analysis, "Impact:") {
		t.Fatalf("expected optimization output, got %q", result.Optimization.Analysis)
	}

	mu.Lock()
	defer mu.Unlock()
	if securityCalls != 1 || optimizationCalls != 1 {
		t.Fatalf("expected both personas called once, got security=%d optimization=%d", securityCalls, optimizationCalls)
	}
}

func sampleBug() model.Bug {
	return model.Bug{
		ID:                42,
		Title:             "XSS in profile render",
		Severity:          5,
		Age:               12,
		BountyValue:       900,
		Reproductions:     4,
		EstimatedFixHours: 5,
		Source:            "github",
		URL:               "https://github.com/OWASP-BLT/BLT-Pages/issues/42",
		Priority:          86.4,
		PriorityBreakdown: model.ScoreBreakdown{
			Severity:      100,
			BountyValue:   90,
			Reproductions: 80,
			Age:           60,
		},
	}
}
