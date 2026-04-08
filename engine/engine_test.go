package engine

import (
	"context"
	"testing"
	"time"

	"bug-bounty-engine/api"
	"bug-bounty-engine/model"
)

func TestEngineRefreshTopKAndFix(t *testing.T) {
	service := api.NewService(
		time.Hour,
		time.Now,
		staticProvider{bugs: []model.Bug{
			{ID: 1, Title: "Low", Severity: 2, Age: 2, BountyValue: 100, Reproductions: 1, EstimatedFixHours: 3, Source: "github"},
			{ID: 2, Title: "High", Severity: 5, Age: 30, BountyValue: 900, Reproductions: 5, EstimatedFixHours: 6, Source: "github"},
		}},
	)

	engine := New(service)
	if _, err := engine.Refresh(context.Background(), false); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	top, err := engine.TopK(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("top-k failed: %v", err)
	}
	if len(top) != 1 || top[0].ID != 2 {
		t.Fatalf("expected top bug id 2, got %+v", top)
	}

	fixed, removed, err := engine.FixByID(context.Background(), 2)
	if err != nil {
		t.Fatalf("fix failed: %v", err)
	}
	if !removed || fixed.ID != 2 {
		t.Fatalf("expected fixed bug id 2, got removed=%v bug=%+v", removed, fixed)
	}

	topAfterFix, err := engine.TopK(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("top-k after fix failed: %v", err)
	}
	if len(topAfterFix) != 1 || topAfterFix[0].ID != 1 {
		t.Fatalf("expected remaining top bug id 1, got %+v", topAfterFix)
	}
}

type staticProvider struct {
	bugs []model.Bug
}

func (p staticProvider) Name() string { return "static" }

func (p staticProvider) Fetch(context.Context) ([]model.Bug, error) {
	return p.bugs, nil
}
