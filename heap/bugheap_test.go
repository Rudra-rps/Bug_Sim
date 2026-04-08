package heap

import (
	"testing"

	"bug-bounty-engine/model"
)

func TestNewPeekAndPopBug(t *testing.T) {
	h := New([]model.Bug{
		{ID: 1, Priority: 44.5},
		{ID: 2, Priority: 91.2},
		{ID: 3, Priority: 67.0},
	})

	peeked, ok := h.Peek()
	if !ok {
		t.Fatal("expected heap to contain items")
	}
	if peeked.ID != 2 {
		t.Fatalf("expected highest priority bug id 2, got %d", peeked.ID)
	}

	popped, ok := h.PopBug()
	if !ok {
		t.Fatal("expected pop to succeed")
	}
	if popped.ID != 2 {
		t.Fatalf("expected popped bug id 2, got %d", popped.ID)
	}

	next, ok := h.Peek()
	if !ok {
		t.Fatal("expected second bug after pop")
	}
	if next.ID != 3 {
		t.Fatalf("expected next bug id 3, got %d", next.ID)
	}
}

func TestTopKPreservesHeapAndOrder(t *testing.T) {
	h := New([]model.Bug{
		{ID: 1, Priority: 10},
		{ID: 2, Priority: 80},
		{ID: 3, Priority: 60},
		{ID: 4, Priority: 95},
	})

	top := h.TopK(3)
	if len(top) != 3 {
		t.Fatalf("expected 3 bugs, got %d", len(top))
	}

	expectedIDs := []int{4, 2, 3}
	for i, expectedID := range expectedIDs {
		if top[i].ID != expectedID {
			t.Fatalf("expected top[%d] bug id %d, got %d", i, expectedID, top[i].ID)
		}
	}

	if h.Len() != 4 {
		t.Fatalf("expected original heap length 4, got %d", h.Len())
	}

	peeked, ok := h.Peek()
	if !ok || peeked.ID != 4 {
		t.Fatalf("expected original heap top to remain bug id 4, got %+v", peeked)
	}
}
