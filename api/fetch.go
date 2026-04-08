 package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"bug-bounty-engine/model"
	"bug-bounty-engine/scorer"
)

const (
	defaultBLTAPIURL      = "https://next.owaspblt.org/api/v1/issues"
	defaultCacheTTL       = 5 * time.Minute
	defaultGitHubPerPage  = 100
	defaultGitHubMaxPages = 3
)

// Provider fetches raw bug data from an upstream source and converts it into
// the shared bug model.
type Provider interface {
	Name() string
	Fetch(ctx context.Context) ([]model.Bug, error)
}

// Config controls provider setup and cache behavior.
type Config struct {
	BLTAPIURL   string
	GitHubOwner string
	GitHubRepo  string
	GitHubLabel string
	GitHubToken string
	CacheTTL    time.Duration
	Now         func() time.Time
}

// Service fetches bugs from upstream providers, normalizes them into the shared
// domain model, scores them, and caches the results for a short period.
type Service struct {
	providers []Provider
	cacheTTL  time.Duration
	now       func() time.Time

	mu       sync.RWMutex
	cached   []model.Bug
	cachedAt time.Time
}

// NewService constructs a fetch service with the supplied providers.
func NewService(cacheTTL time.Duration, now func() time.Time, providers ...Provider) *Service {
	if cacheTTL <= 0 {
		cacheTTL = defaultCacheTTL
	}
	if now == nil {
		now = time.Now
	}

	return &Service{
		providers: providers,
		cacheTTL:  cacheTTL,
		now:       now,
	}
}

// NewDefaultServiceFromEnv configures GitHub issues from OWASP-BLT/BLT-Pages
// as the default source of real-time bug data.
func NewDefaultServiceFromEnv(client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	cfg := Config{
		BLTAPIURL:   strings.TrimSpace(os.Getenv("BLT_API_URL")),
		GitHubOwner: strings.TrimSpace(os.Getenv("GITHUB_OWNER")),
		GitHubRepo:  strings.TrimSpace(os.Getenv("GITHUB_REPO")),
		GitHubLabel: strings.TrimSpace(os.Getenv("GITHUB_LABEL")),
		GitHubToken: strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		CacheTTL:    defaultCacheTTL,
		Now:         time.Now,
	}

	if cfg.BLTAPIURL == "" {
		cfg.BLTAPIURL = defaultBLTAPIURL
	}
	if cfg.GitHubOwner == "" {
		cfg.GitHubOwner = "OWASP-BLT"
	}
	if cfg.GitHubRepo == "" {
		cfg.GitHubRepo = "BLT-Pages"
	}
	if cfg.GitHubLabel == "" {
		cfg.GitHubLabel = "bug"
	}

	return NewService(
		cfg.CacheTTL,
		cfg.Now,
		NewGitHubProvider(client, cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubLabel, cfg.GitHubToken, cfg.Now),
	)
}

// FetchBugs returns scored bugs, optionally forcing a refresh instead of
// serving the cached result.
func (s *Service) FetchBugs(ctx context.Context, forceRefresh bool) ([]model.Bug, error) {
	if !forceRefresh {
		if bugs, ok := s.cachedBugs(); ok {
			return bugs, nil
		}
	}

	var errs []error
	for _, provider := range s.providers {
		if provider == nil {
			continue
		}

		bugs, err := provider.Fetch(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", provider.Name(), err))
			continue
		}
		if len(bugs) == 0 {
			errs = append(errs, fmt.Errorf("%s: no bugs returned", provider.Name()))
			continue
		}

		scored := scorer.ScoreBugs(bugs)
		s.storeCache(scored)
		return cloneBugs(scored), nil
	}

	if len(errs) == 0 {
		return nil, errors.New("no providers configured")
	}
	return nil, errors.Join(errs...)
}

func (s *Service) cachedBugs() ([]model.Bug, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cached) == 0 {
		return nil, false
	}
	if s.now().Sub(s.cachedAt) > s.cacheTTL {
		return nil, false
	}
	return cloneBugs(s.cached), true
}

func (s *Service) storeCache(bugs []model.Bug) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cached = cloneBugs(bugs)
	s.cachedAt = s.now()
}

func cloneBugs(bugs []model.Bug) []model.Bug {
	if len(bugs) == 0 {
		return nil
	}
	out := make([]model.Bug, len(bugs))
	copy(out, bugs)
	return out
}

// BLTProvider fetches bugs from a BLT-compatible JSON endpoint. The URL can be
// either a collection endpoint or a base URL; multiple common issue paths are
// tried to improve resilience when the upstream contract differs.
type BLTProvider struct {
	client *http.Client
	apiURL string
	now    func() time.Time
}

func NewBLTProvider(client *http.Client, apiURL string, now func() time.Time) *BLTProvider {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if apiURL == "" {
		apiURL = defaultBLTAPIURL
	}
	if now == nil {
		now = time.Now
	}

	return &BLTProvider{
		client: client,
		apiURL: apiURL,
		now:    now,
	}
}

func (p *BLTProvider) Name() string { return "blt" }

func (p *BLTProvider) Fetch(ctx context.Context) ([]model.Bug, error) {
	var errs []error
	for _, endpoint := range candidateBLTEndpoints(p.apiURL) {
		bugs, err := p.fetchEndpoint(ctx, endpoint)
		if err == nil && len(bugs) > 0 {
			return bugs, nil
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", endpoint, err))
		}
	}

	if len(errs) == 0 {
		return nil, errors.New("no BLT endpoints produced results")
	}
	return nil, errors.Join(errs...)
}

func (p *BLTProvider) fetchEndpoint(ctx context.Context, endpoint string) ([]model.Bug, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	issues, err := decodeBLTIssues(raw)
	if err != nil {
		return nil, err
	}

	now := p.now()
	bugs := make([]model.Bug, 0, len(issues))
	for _, issue := range issues {
		title := firstNonEmpty(issue.Title, issue.Name, issue.Summary)
		if title == "" {
			continue
		}

		bugs = append(bugs, model.Bug{
			ID:                issue.ID,
			Title:             title,
			Severity:          normalizeSeverity(issue.Severity),
			Age:               ageInDays(now, issue.CreatedAt),
			BountyValue:       chooseBounty(issue.BountyValue, issue.Reward, issue.Points),
			Reproductions:     chooseReproductions(issue.Reproductions, issue.Confirmations, issue.Upvotes),
			EstimatedFixHours: chooseFixHours(issue.EstimatedFixHours, issue.HoursToFix, issue.Complexity, normalizeSeverity(issue.Severity)),
			Source:            "blt",
			URL:               firstNonEmpty(issue.URL, issue.HTMLURL),
		})
	}
	return bugs, nil
}

type bltIssue struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Name              string `json:"name"`
	Summary           string `json:"summary"`
	Severity          any    `json:"severity"`
	BountyValue       int    `json:"bountyValue"`
	Reward            int    `json:"reward"`
	Points            int    `json:"points"`
	Reproductions     int    `json:"reproductions"`
	Confirmations     int    `json:"confirmations"`
	Upvotes           int    `json:"upvotes"`
	EstimatedFixHours int    `json:"estimatedFixHours"`
	HoursToFix        int    `json:"hoursToFix"`
	Complexity        int    `json:"complexity"`
	CreatedAt         string `json:"createdAt"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
}

type bltListEnvelope struct {
	Results []bltIssue `json:"results"`
	Issues  []bltIssue `json:"issues"`
	Data    []bltIssue `json:"data"`
}

func candidateBLTEndpoints(base string) []string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultBLTAPIURL
	}

	u, err := url.Parse(base)
	if err != nil {
		return []string{base}
	}

	if looksLikeCollectionPath(u.Path) {
		return []string{u.String()}
	}

	paths := []string{
		"/api/v1/issues",
		"/api/issues",
		"/issues.json",
	}

	endpoints := make([]string, 0, len(paths))
	for _, path := range paths {
		next := *u
		next.Path = strings.TrimRight(u.Path, "/") + path
		endpoints = append(endpoints, next.String())
	}
	return endpoints
}

func looksLikeCollectionPath(path string) bool {
	path = strings.ToLower(path)
	return strings.Contains(path, "issue") || strings.HasSuffix(path, ".json")
}

func decodeBLTIssues(raw []byte) ([]bltIssue, error) {
	var direct []bltIssue
	if err := json.Unmarshal(raw, &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapped bltListEnvelope
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, err
	}

	switch {
	case len(wrapped.Results) > 0:
		return wrapped.Results, nil
	case len(wrapped.Issues) > 0:
		return wrapped.Issues, nil
	case len(wrapped.Data) > 0:
		return wrapped.Data, nil
	default:
		return nil, errors.New("BLT response did not contain issue records")
	}
}

// GitHubProvider fetches open issues from GitHub and maps them into bugs.
type GitHubProvider struct {
	client   *http.Client
	owner    string
	repo     string
	label    string
	token    string
	maxPages int
	now      func() time.Time
}

func NewGitHubProvider(client *http.Client, owner, repo, label, token string, now func() time.Time) *GitHubProvider {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if now == nil {
		now = time.Now
	}

	return &GitHubProvider{
		client:   client,
		owner:    owner,
		repo:     repo,
		label:    label,
		token:    token,
		maxPages: defaultGitHubMaxPages,
		now:      now,
	}
}

func (p *GitHubProvider) Name() string { return "github" }

func (p *GitHubProvider) Fetch(ctx context.Context) ([]model.Bug, error) {
	if p.owner == "" || p.repo == "" {
		return nil, errors.New("owner and repo must be configured")
	}

	now := p.now()
	bugs := make([]model.Bug, 0, defaultGitHubPerPage)

	for page := 1; page <= p.maxPages; page++ {
		issues, err := p.fetchIssuesPage(ctx, page)
		if err != nil {
			return nil, err
		}

		for _, issue := range issues {
			if issue.PullRequest.URL != "" {
				continue
			}

			severity := severityFromGitHubLabels(issue.Labels)
			reproductions := reproductionsFromComments(issue.Comments)
			bugs = append(bugs, model.Bug{
				ID:                issue.Number,
				Title:             issue.Title,
				Severity:          severity,
				Age:               ageInDays(now, issue.CreatedAt),
				BountyValue:       bountyFromGitHubLabels(issue.Labels, severity),
				Reproductions:     reproductions,
				EstimatedFixHours: chooseFixHours(0, 0, 0, severity),
				Source:            "github",
				URL:               issue.HTMLURL,
			})
		}

		if len(issues) < defaultGitHubPerPage {
			break
		}
	}

	return bugs, nil
}

func (p *GitHubProvider) fetchIssuesPage(ctx context.Context, page int) ([]githubIssue, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("sort", "created")
	query.Set("direction", "desc")
	query.Set("per_page", strconv.Itoa(defaultGitHubPerPage))
	query.Set("page", strconv.Itoa(page))
	if strings.TrimSpace(p.label) != "" {
		query.Set("labels", p.label)
	}

	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?%s", p.owner, p.repo, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var issues []githubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, err
	}

	return issues, nil
}

type githubIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   string        `json:"created_at"`
	Comments    int           `json:"comments"`
	Labels      []githubLabel `json:"labels"`
	PullRequest struct {
		URL string `json:"url"`
	} `json:"pull_request"`
}

type githubLabel struct {
	Name string `json:"name"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeSeverity(raw any) int {
	switch value := raw.(type) {
	case nil:
		return 3
	case float64:
		return clampInt(int(value), 1, 5)
	case int:
		return clampInt(value, 1, 5)
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "critical", "sev-0", "sev0", "p0":
			return 5
		case "high", "sev-1", "sev1", "p1":
			return 4
		case "medium", "moderate", "sev-2", "sev2", "p2":
			return 3
		case "low", "sev-3", "sev3", "p3":
			return 2
		case "info", "informational", "trivial", "sev-4", "sev4", "p4":
			return 1
		default:
			if parsed, err := strconv.Atoi(normalized); err == nil {
				return clampInt(parsed, 1, 5)
			}
		}
	}
	return 3
}

func ageInDays(now time.Time, createdAt string) int {
	if strings.TrimSpace(createdAt) == "" {
		return 0
	}

	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return 0
	}
	days := int(now.Sub(parsed).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func chooseBounty(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 100
}

func chooseReproductions(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 1
}

func chooseFixHours(explicit int, fallback int, complexity int, severity int) int {
	for _, value := range []int{explicit, fallback} {
		if value > 0 {
			return value
		}
	}
	if complexity > 0 {
		return clampInt(complexity*2, 1, 16)
	}
	return clampInt(severity*2, 2, 12)
}

func severityFromGitHubLabels(labels []githubLabel) int {
	for _, label := range labels {
		name := strings.ToLower(label.Name)
		switch {
		case strings.Contains(name, "critical"), strings.Contains(name, "sev:critical"), strings.Contains(name, "severity:critical"):
			return 5
		case strings.Contains(name, "high"), strings.Contains(name, "sev:high"), strings.Contains(name, "severity:high"):
			return 4
		case strings.Contains(name, "medium"), strings.Contains(name, "moderate"), strings.Contains(name, "sev:medium"):
			return 3
		case strings.Contains(name, "low"), strings.Contains(name, "sev:low"):
			return 2
		case strings.Contains(name, "info"), strings.Contains(name, "good first issue"):
			return 1
		}
	}
	return 3
}

func bountyFromGitHubLabels(labels []githubLabel, severity int) int {
	for _, label := range labels {
		name := strings.ToLower(strings.TrimSpace(label.Name))
		if strings.Contains(name, "$") {
			for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r < '0' || r > '9' }) {
				if value, err := strconv.Atoi(part); err == nil && value > 0 {
					return value
				}
			}
		}
	}

	switch severity {
	case 5:
		return 1000
	case 4:
		return 750
	case 3:
		return 400
	case 2:
		return 200
	default:
		return 100
	}
}

func reproductionsFromComments(comments int) int {
	if comments <= 0 {
		return 1
	}
	return comments + 1
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
