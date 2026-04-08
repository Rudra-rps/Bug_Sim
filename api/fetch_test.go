package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bug-bounty-engine/model"
)

func TestServiceFallsBackToSecondProviderAndScores(t *testing.T) {
	now := fixedNow()
	service := NewService(
		time.Minute,
		func() time.Time { return now },
		stubProvider{name: "blt", err: errors.New("upstream failed")},
		stubProvider{name: "github", bugs: []model.Bug{
			{ID: 1, Title: "Medium issue", Severity: 3, Age: 5, BountyValue: 300, Reproductions: 2, EstimatedFixHours: 4, Source: "github"},
			{ID: 2, Title: "Critical issue", Severity: 5, Age: 10, BountyValue: 900, Reproductions: 5, EstimatedFixHours: 6, Source: "github"},
		}},
	)

	bugs, err := service.FetchBugs(context.Background(), false)
	if err != nil {
		t.Fatalf("expected successful fallback fetch, got error: %v", err)
	}
	if len(bugs) != 2 {
		t.Fatalf("expected 2 bugs, got %d", len(bugs))
	}
	if bugs[1].Priority <= bugs[0].Priority {
		t.Fatalf("expected second bug to score higher, got %.2f <= %.2f", bugs[1].Priority, bugs[0].Priority)
	}
	if bugs[1].Source != "github" {
		t.Fatalf("expected fallback provider source github, got %q", bugs[1].Source)
	}
}

func TestServiceUsesCacheWhenFresh(t *testing.T) {
	now := fixedNow()
	provider := &countingProvider{bugs: []model.Bug{
		{ID: 1, Title: "Bug", Severity: 3, Age: 4, BountyValue: 200, Reproductions: 2, EstimatedFixHours: 4},
	}}
	service := NewService(time.Hour, func() time.Time { return now }, provider)

	_, err := service.FetchBugs(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected first fetch error: %v", err)
	}
	_, err = service.FetchBugs(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected second fetch error: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once due to caching, got %d", provider.calls)
	}
}

func TestBLTProviderParsesWrappedPayload(t *testing.T) {
	now := fixedNow()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"id": 7,
					"title": "XSS in markdown preview",
					"severity": "high",
					"reward": 650,
					"confirmations": 3,
					"estimatedFixHours": 5,
					"createdAt": "2026-04-01T00:00:00Z",
					"url": "https://next.owaspblt.org/issues/7"
				}
			]
		}`))
	}))
	defer server.Close()

	provider := NewBLTProvider(server.Client(), server.URL, func() time.Time { return now })
	bugs, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected BLT fetch to succeed, got %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("expected 1 bug, got %d", len(bugs))
	}
	bug := bugs[0]
	if bug.ID != 7 || bug.Severity != 4 || bug.BountyValue != 650 || bug.Reproductions != 3 || bug.EstimatedFixHours != 5 {
		t.Fatalf("unexpected bug mapping: %+v", bug)
	}
	if bug.Age != 7 {
		t.Fatalf("expected age 7, got %d", bug.Age)
	}
}

func TestGitHubProviderParsesIssuesAndSkipsPullRequests(t *testing.T) {
	now := fixedNow()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != "open" || q.Get("labels") != "bug" || q.Get("sort") != "created" || q.Get("direction") != "desc" {
			t.Fatalf("unexpected query params: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"number": 101,
				"title": "Critical auth bug",
				"html_url": "https://github.com/example/repo/issues/101",
				"created_at": "2026-04-05T00:00:00Z",
				"comments": 4,
				"labels": [
					{"name": "severity:critical"},
					{"name": "$1200 bounty"}
				]
			},
			{
				"number": 102,
				"title": "This is actually a PR",
				"html_url": "https://github.com/example/repo/pull/102",
				"created_at": "2026-04-05T00:00:00Z",
				"comments": 1,
				"labels": [],
				"pull_request": {"url": "https://api.github.com/repos/example/repo/pulls/102"}
			}
		]`))
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = rewriteTransport{base: http.DefaultTransport, target: server.URL}

	provider := NewGitHubProvider(client, "example", "repo", "bug", "", func() time.Time { return now })
	provider.maxPages = 1
	bugs, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected GitHub fetch to succeed, got %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("expected 1 issue after skipping PRs, got %d", len(bugs))
	}
	bug := bugs[0]
	if bug.ID != 101 || bug.Severity != 5 || bug.BountyValue != 1200 || bug.Reproductions != 5 {
		t.Fatalf("unexpected GitHub mapping: %+v", bug)
	}
	if bug.Age != 3 {
		t.Fatalf("expected age 3, got %d", bug.Age)
	}
}

type stubProvider struct {
	name string
	bugs []model.Bug
	err  error
}

func (p stubProvider) Name() string { return p.name }

func (p stubProvider) Fetch(context.Context) ([]model.Bug, error) {
	return p.bugs, p.err
}

type countingProvider struct {
	calls int
	bugs  []model.Bug
}

func (p *countingProvider) Name() string { return "counting" }

func (p *countingProvider) Fetch(context.Context) ([]model.Bug, error) {
	p.calls++
	return p.bugs, nil
}

type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, err := url.Parse(t.target)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	return t.base.RoundTrip(req)
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
}
