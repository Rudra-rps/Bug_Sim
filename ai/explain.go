package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"bug-bounty-engine/model"
)

const (
	defaultGroqAPIURL     = "https://api.groq.com/openai/v1/chat/completions"
	defaultGroqModel      = "llama-3.1-8b-instant"
	defaultGroqTimeout    = 12 * time.Second
	defaultTemperature    = 0.2
	defaultMaxTokens      = 220
	securityAgentName     = "Sentinel"
	optimizationAgentName = "Optimizer"
)

// AgentExplanation stores one persona's explanation.
type AgentExplanation struct {
	Name     string `json:"name"`
	Focus    string `json:"focus"`
	Analysis string `json:"analysis"`
}

// ExplainResult keeps backwards-compatible summary/detail fields while also
// exposing persona-specific agent output for the frontend.
type ExplainResult struct {
	Summary      string           `json:"summary"`
	Detail       string           `json:"detail"`
	Security     AgentExplanation `json:"security"`
	Optimization AgentExplanation `json:"optimization"`
	Provider     string           `json:"provider"`
	Model        string           `json:"model"`
	Fallback     bool             `json:"fallback"`
	Reason       string           `json:"reason,omitempty"`
}

// GroqExplainer calls Groq Chat Completions for two persona agents in
// parallel: one security-focused and one optimization-focused.
type GroqExplainer struct {
	client      *http.Client
	apiURL      string
	apiKey      string
	model       string
	timeout     time.Duration
	temperature float64
	maxTokens   int
}

func NewGroqMultiAgentFromEnv(client *http.Client) *GroqExplainer {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	return &GroqExplainer{
		client:      client,
		apiURL:      stringsOrDefault(strings.TrimSpace(os.Getenv("GROQ_API_URL")), defaultGroqAPIURL),
		apiKey:      strings.TrimSpace(os.Getenv("GROQ_API_KEY")),
		model:       stringsOrDefault(strings.TrimSpace(os.Getenv("GROQ_MODEL")), defaultGroqModel),
		timeout:     durationFromEnv("GROQ_TIMEOUT_SEC", defaultGroqTimeout),
		temperature: floatFromEnv("GROQ_TEMPERATURE", defaultTemperature),
		maxTokens:   intFromEnv("GROQ_MAX_TOKENS", defaultMaxTokens),
	}
}

func (e *GroqExplainer) Explain(ctx context.Context, bug model.Bug) ExplainResult {
	base := deterministicResult(bug, "groq", e.model, "GROQ_API_KEY is not configured")
	if strings.TrimSpace(e.apiKey) == "" {
		return base
	}

	userPrompt := buildUserPrompt(bug)
	securityOut := base.Security.Analysis
	optimizationOut := base.Optimization.Analysis

	var securityErr error
	var optimizationErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		securityOut, securityErr = e.runPersona(ctx, securitySystemPrompt, userPrompt)
	}()

	go func() {
		defer wg.Done()
		optimizationOut, optimizationErr = e.runPersona(ctx, optimizationSystemPrompt, userPrompt)
	}()

	wg.Wait()

	var errs []error
	if securityErr != nil {
		errs = append(errs, fmt.Errorf("security agent: %w", securityErr))
		securityOut = base.Security.Analysis
	}
	if optimizationErr != nil {
		errs = append(errs, fmt.Errorf("optimization agent: %w", optimizationErr))
		optimizationOut = base.Optimization.Analysis
	}

	reason := ""
	fallback := len(errs) > 0
	if fallback {
		if joined := errors.Join(errs...); joined != nil {
			reason = strings.TrimSpace(joined.Error())
		}
	}

	return ExplainResult{
		Summary: buildSummary(bug, fallback),
		Detail:  buildDetail(securityOut, optimizationOut),
		Security: AgentExplanation{
			Name:     securityAgentName,
			Focus:    "security",
			Analysis: securityOut,
		},
		Optimization: AgentExplanation{
			Name:     optimizationAgentName,
			Focus:    "optimization",
			Analysis: optimizationOut,
		},
		Provider: "groq",
		Model:    e.model,
		Fallback: fallback,
		Reason:   reason,
	}
}

func (e *GroqExplainer) runPersona(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	content, err := e.callChatCompletion(timeoutCtx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("empty response from model")
	}
	return truncate(content, 700), nil
}

func (e *GroqExplainer) callChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := groqChatRequest{
		Model: e.model,
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: e.temperature,
		MaxTokens:   e.maxTokens,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("groq status %d: %s", resp.StatusCode, extractAPIError(body))
	}

	var parsed groqChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("no completion choices returned")
	}

	return parsed.Choices[0].Message.Content, nil
}

func deterministicResult(bug model.Bug, provider, modelName, reason string) ExplainResult {
	security := deterministicSecurityAnalysis(bug)
	optimization := deterministicOptimizationAnalysis(bug)
	return ExplainResult{
		Summary: buildSummary(bug, true),
		Detail:  buildDetail(security, optimization),
		Security: AgentExplanation{
			Name:     securityAgentName,
			Focus:    "security",
			Analysis: security,
		},
		Optimization: AgentExplanation{
			Name:     optimizationAgentName,
			Focus:    "optimization",
			Analysis: optimization,
		},
		Provider: provider,
		Model:    modelName,
		Fallback: true,
		Reason:   strings.TrimSpace(reason),
	}
}

func buildSummary(bug model.Bug, fallback bool) string {
	source := "Groq multi-agent analysis"
	if fallback {
		source = "Deterministic fallback analysis"
	}
	return fmt.Sprintf(
		"%s: %s risk remains high at priority %.1f and should be queued in the next %d-hour planning window.",
		source,
		severityLabel(bug.Severity),
		bug.Priority,
		bug.EstimatedFixHours,
	)
}

func buildDetail(security, optimization string) string {
	return "Security agent (" + securityAgentName + "): " + security + "\n\n" +
		"Optimization agent (" + optimizationAgentName + "): " + optimization
}

func deterministicSecurityAnalysis(bug model.Bug) string {
	return fmt.Sprintf(
		"Risk=%s. Exploitability increases with %d reproductions and age %d days. Next fix should harden affected path and add a regression test immediately.",
		severityLabel(bug.Severity),
		bug.Reproductions,
		bug.Age,
	)
}

func deterministicOptimizationAnalysis(bug model.Bug) string {
	return fmt.Sprintf(
		"Impact-per-hour is high: priority %.1f over %dh estimated fix time. Plan: patch highest-risk path first, then ship minimal verification to keep cycle time low.",
		bug.Priority,
		bug.EstimatedFixHours,
	)
}

func buildUserPrompt(bug model.Bug) string {
	payload := map[string]any{
		"id":                bug.ID,
		"title":             bug.Title,
		"severity":          bug.Severity,
		"age":               bug.Age,
		"bountyValue":       bug.BountyValue,
		"reproductions":     bug.Reproductions,
		"estimatedFixHours": bug.EstimatedFixHours,
		"priority":          bug.Priority,
		"priorityBreakdown": map[string]float64{
			"severity":      bug.PriorityBreakdown.Severity,
			"bountyValue":   bug.PriorityBreakdown.BountyValue,
			"reproductions": bug.PriorityBreakdown.Reproductions,
			"age":           bug.PriorityBreakdown.Age,
		},
		"url": bug.URL,
	}
	raw, _ := json.Marshal(payload)
	return "Analyze this bug and output plain text only. Keep it under 90 words.\n" +
		"Bug JSON: " + string(raw)
}

func extractAPIError(body []byte) string {
	message := strings.TrimSpace(string(body))
	if message == "" {
		return "unknown error"
	}

	var parsed groqChatResponse
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil {
		if trimmed := strings.TrimSpace(parsed.Error.Message); trimmed != "" {
			return trimmed
		}
	}
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func severityLabel(severity int) string {
	switch {
	case severity >= 5:
		return "critical"
	case severity == 4:
		return "high"
	case severity == 3:
		return "medium"
	case severity == 2:
		return "low"
	default:
		return "informational"
	}
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max]) + "..."
}

func stringsOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func intFromEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func floatFromEnv(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	seconds := intFromEnv(name, int(fallback.Seconds()))
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

const securitySystemPrompt = `You are Sentinel, a senior security engineer.
Give actionable security triage for this single bug.
Output exactly 3 short lines in this format:
Risk: ...
Exploitability: ...
Next Fix: ...
No markdown and no bullet symbols.`

const optimizationSystemPrompt = `You are Optimizer, a senior engineering efficiency strategist.
Give actionable optimization triage for this single bug.
Output exactly 3 short lines in this format:
Impact: ...
Effort: ...
Plan: ...
No markdown and no bullet symbols.`

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqChatRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type groqChatResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
