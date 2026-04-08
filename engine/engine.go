package engine

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"bug-bounty-engine/api"
	bugheap "bug-bounty-engine/heap"
	"bug-bounty-engine/model"
	"bug-bounty-engine/scheduler"
)

const defaultTopK = 5

type Engine struct {
	fetcher *api.Service

	mu          sync.RWMutex
	bugsByID    map[int]model.Bug
	heap        *bugheap.BugHeap
	lastRefresh time.Time
}

func New(fetcher *api.Service) *Engine {
	return &Engine{
		fetcher:  fetcher,
		bugsByID: make(map[int]model.Bug),
		heap:     bugheap.New(nil),
	}
}

func (e *Engine) Refresh(ctx context.Context, force bool) ([]model.Bug, error) {
	if e.fetcher == nil {
		return nil, errors.New("fetch service is not configured")
	}

	bugs, err := e.fetcher.FetchBugs(ctx, force)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.loadLocked(bugs)
	return e.sortedBugsLocked(), nil
}

func (e *Engine) All(ctx context.Context, forceRefresh bool) ([]model.Bug, error) {
	if forceRefresh {
		return e.Refresh(ctx, true)
	}
	if err := e.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sortedBugsLocked(), nil
}

func (e *Engine) TopK(ctx context.Context, k int, forceRefresh bool) ([]model.Bug, error) {
	if forceRefresh {
		if _, err := e.Refresh(ctx, true); err != nil {
			return nil, err
		}
	} else if err := e.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	if k <= 0 {
		k = defaultTopK
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.heap.TopK(k), nil
}

func (e *Engine) FixByID(ctx context.Context, id int) (model.Bug, bool, error) {
	if err := e.ensureLoaded(ctx); err != nil {
		return model.Bug{}, false, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	bug, ok := e.bugsByID[id]
	if !ok {
		return model.Bug{}, false, nil
	}

	delete(e.bugsByID, id)
	e.rebuildHeapLocked()
	return bug, true, nil
}

func (e *Engine) GetByID(ctx context.Context, id int) (model.Bug, bool, error) {
	if err := e.ensureLoaded(ctx); err != nil {
		return model.Bug{}, false, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	bug, ok := e.bugsByID[id]
	if !ok {
		return model.Bug{}, false, nil
	}
	return bug, true, nil
}

func (e *Engine) Schedule(ctx context.Context, hours int) (scheduler.ScheduleResult, error) {
	if err := e.ensureLoaded(ctx); err != nil {
		return scheduler.ScheduleResult{}, err
	}

	e.mu.RLock()
	bugs := e.sliceLocked()
	e.mu.RUnlock()
	return scheduler.Greedy(bugs, hours), nil
}

func (e *Engine) Compare(ctx context.Context, hours int, cap int) (scheduler.CompareResult, error) {
	if err := e.ensureLoaded(ctx); err != nil {
		return scheduler.CompareResult{}, err
	}

	e.mu.RLock()
	bugs := e.sliceLocked()
	e.mu.RUnlock()
	return scheduler.Compare(bugs, hours, cap), nil
}

func (e *Engine) LastRefresh() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastRefresh
}

func (e *Engine) ensureLoaded(ctx context.Context) error {
	e.mu.RLock()
	loaded := len(e.bugsByID) > 0
	e.mu.RUnlock()
	if loaded {
		return nil
	}

	_, err := e.Refresh(ctx, false)
	return err
}

func (e *Engine) loadLocked(bugs []model.Bug) {
	e.bugsByID = make(map[int]model.Bug, len(bugs))
	for _, bug := range bugs {
		e.bugsByID[bug.ID] = bug
	}
	e.rebuildHeapLocked()
	e.lastRefresh = time.Now()
}

func (e *Engine) rebuildHeapLocked() {
	e.heap = bugheap.New(e.sliceLocked())
}

func (e *Engine) sliceLocked() []model.Bug {
	bugs := make([]model.Bug, 0, len(e.bugsByID))
	for _, bug := range e.bugsByID {
		bugs = append(bugs, bug)
	}
	return bugs
}

func (e *Engine) sortedBugsLocked() []model.Bug {
	bugs := e.sliceLocked()
	sort.SliceStable(bugs, func(i, j int) bool {
		return bugs[i].Priority > bugs[j].Priority
	})
	return bugs
}
