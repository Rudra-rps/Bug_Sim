package heap

import (
	stdheap "container/heap"

	"bug-bounty-engine/model"
)

// BugHeap implements a max-heap ordered by bug priority.
type BugHeap []model.Bug

func (h BugHeap) Len() int { return len(h) }

func (h BugHeap) Less(i, j int) bool {
	return h[i].Priority > h[j].Priority
}

func (h BugHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *BugHeap) Push(x any) {
	*h = append(*h, x.(model.Bug))
}

func (h *BugHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// New builds and initializes a max-heap from the provided bugs.
func New(bugs []model.Bug) *BugHeap {
	h := BugHeap(append([]model.Bug(nil), bugs...))
	stdheap.Init(&h)
	return &h
}

// Peek returns the current highest-priority bug without removing it.
func (h *BugHeap) Peek() (model.Bug, bool) {
	if h.Len() == 0 {
		return model.Bug{}, false
	}
	return (*h)[0], true
}

// PopBug removes and returns the current highest-priority bug.
func (h *BugHeap) PopBug() (model.Bug, bool) {
	if h.Len() == 0 {
		return model.Bug{}, false
	}
	return stdheap.Pop(h).(model.Bug), true
}

// PushBug inserts a scored bug into the heap.
func (h *BugHeap) PushBug(bug model.Bug) {
	stdheap.Push(h, bug)
}

// TopK returns the top k bugs without mutating the original heap.
func (h *BugHeap) TopK(k int) []model.Bug {
	if k <= 0 || h.Len() == 0 {
		return nil
	}

	clone := BugHeap(append([]model.Bug(nil), (*h)...))
	stdheap.Init(&clone)

	top := make([]model.Bug, 0, min(k, clone.Len()))
	for i := 0; i < k && clone.Len() > 0; i++ {
		top = append(top, stdheap.Pop(&clone).(model.Bug))
	}
	return top
}
