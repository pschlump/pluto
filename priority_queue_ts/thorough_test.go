// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package priority_queue_ts

import (
	"fmt"
	"testing"
)

// TestNlAPIUnderLock exercises every Nl-prefixed (no-lock) method while
// holding the queue's write lock, covering both empty and non-empty states
// and valid/invalid positions.
func TestNlAPIUnderLock(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()

	// Empty queue under the lock.
	pq.Lock()
	if !pq.NlIsEmpty() {
		t.Error("NlIsEmpty() = false, want true on new queue")
	}
	if pq.NlLen() != 0 {
		t.Errorf("NlLen() = %d, want 0", pq.NlLen())
	}
	if v := pq.NlPeek(); v != nil {
		t.Errorf("NlPeek() on empty queue = %v, want nil", v)
	}
	if v := pq.NlPop(); v != nil {
		t.Errorf("NlPop() on empty queue = %v, want nil", v)
	}
	if rv, pos, err := pq.NlSearch(&PqTest{priority: 1}); err == nil || rv != nil || pos != -1 {
		t.Errorf("NlSearch() on empty queue = (%v, %d, %v), want (nil, -1, error)", rv, pos, err)
	}
	if ok := pq.NlUpdatePriority(0, &PqTest{priority: 1}); ok {
		t.Error("NlUpdatePriority(0) on empty queue = true, want false")
	}
	if err := pq.NlDelete(0); err == nil {
		t.Error("NlDelete(0) on empty queue should return an error")
	}
	pq.Unlock()

	// Non-empty queue under the lock.
	pq.Lock()
	pq.NlInsert(&PqTest{value: "banana", priority: 3})
	pq.NlInsert(&PqTest{value: "apple", priority: 2})
	pq.NlInsert(&PqTest{value: "pear", priority: 4})
	if pq.NlIsEmpty() {
		t.Error("NlIsEmpty() = true after 3 NlInsert, want false")
	}
	if pq.NlLen() != 3 {
		t.Errorf("NlLen() = %d, want 3", pq.NlLen())
	}
	if v := pq.NlPeek(); v == nil || v.value != "apple" {
		t.Errorf("NlPeek() = %v, want apple (priority 2)", v)
	}
	if pq.NlLen() != 3 {
		t.Errorf("NlLen() after NlPeek = %d, want 3 (peek must not remove)", pq.NlLen())
	}

	// NlSearch: hit and miss.
	rv, pos, err := pq.NlSearch(&PqTest{priority: 3})
	if err != nil || rv == nil || rv.value != "banana" {
		t.Errorf("NlSearch(3) = (%v, %d, %v), want banana", rv, pos, err)
	}
	if pos < 0 || pos >= pq.NlLen() {
		t.Errorf("NlSearch(3) pos = %d, want in range [0..%d)", pos, pq.NlLen())
	}
	rv, pos, err = pq.NlSearch(&PqTest{priority: 99})
	if err == nil || rv != nil || pos != -1 {
		t.Errorf("NlSearch(99) = (%v, %d, %v), want (nil, -1, error)", rv, pos, err)
	}

	// NlUpdatePriority: valid position moves the element; invalid rejected.
	_, pos, _ = pq.NlSearch(&PqTest{priority: 2})
	if !pq.NlUpdatePriority(pos, &PqTest{value: "apple", priority: 9}) {
		t.Errorf("NlUpdatePriority(%d) = false, want true", pos)
	}
	if v := pq.NlPeek(); v == nil || v.value != "banana" {
		t.Errorf("NlPeek() after NlUpdatePriority = %v, want banana", v)
	}
	if ok := pq.NlUpdatePriority(-1, &PqTest{priority: 1}); ok {
		t.Error("NlUpdatePriority(-1) = true, want false")
	}
	if ok := pq.NlUpdatePriority(pq.NlLen(), &PqTest{priority: 1}); ok {
		t.Error("NlUpdatePriority(NlLen()) = true, want false")
	}

	// NlDelete: valid position removes; invalid rejected.
	_, pos, _ = pq.NlSearch(&PqTest{priority: 4})
	if err := pq.NlDelete(pos); err != nil {
		t.Errorf("NlDelete(%d) error = %v, want nil", pos, err)
	}
	if pq.NlLen() != 2 {
		t.Errorf("NlLen() after NlDelete = %d, want 2", pq.NlLen())
	}
	if err := pq.NlDelete(-1); err == nil {
		t.Error("NlDelete(-1) should return an error")
	}
	if err := pq.NlDelete(pq.NlLen()); err == nil {
		t.Error("NlDelete(NlLen()) should return an error")
	}

	// NlPop drains in priority order.
	want := []string{"banana", "apple"}
	for i, w := range want {
		got := pq.NlPop()
		if got == nil || got.value != w {
			t.Errorf("NlPop() #%d = %v, want %q", i, got, w)
		}
	}
	if !pq.NlIsEmpty() {
		t.Error("NlIsEmpty() = false after draining, want true")
	}
	pq.Unlock()

	// The queue must be fully usable again after Unlock.
	pq.Insert(&PqTest{value: "fig", priority: 5})
	if pq.Len() != 1 {
		t.Errorf("Len() after Insert post-Unlock = %d, want 1", pq.Len())
	}
}

// TestNlAtomicSearchThenUpdate verifies the documented use of Lock/Unlock:
// an atomic Search-then-UpdatePriority sequence using the Nl methods.
func TestNlAtomicSearchThenUpdate(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 10},
		PqTest{value: "b", priority: 20},
		PqTest{value: "c", priority: 30},
	)

	pq.Lock()
	_, pos, err := pq.NlSearch(&PqTest{priority: 30})
	if err != nil {
		pq.Unlock()
		t.Fatalf("NlSearch(30) error = %v", err)
	}
	if !pq.NlUpdatePriority(pos, &PqTest{value: "c", priority: 1}) {
		pq.Unlock()
		t.Fatalf("NlUpdatePriority(%d) = false, want true", pos)
	}
	pq.Unlock()

	wantOrder := []int{1, 10, 20}
	for i, want := range wantOrder {
		got := pq.Pop()
		if got == nil || got.priority != want {
			t.Fatalf("Pop() #%d = %v, want priority %d", i, got, want)
		}
	}
}

// TestAllMutationInsideLoop verifies the documented guarantee that it is
// safe to call queue methods from inside the All loop body (the iterator
// works on a snapshot).
func TestAllMutationInsideLoop(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "b", priority: 2},
		PqTest{value: "a", priority: 1},
		PqTest{value: "c", priority: 3},
	)

	count := 0
	for v := range pq.All() {
		count++
		// Mutate the live queue while iterating the snapshot.
		pq.Insert(&PqTest{value: fmt.Sprintf("new%d", count), priority: 100 + count})
		if v.priority > 100 {
			t.Errorf("All() yielded snapshot-affected element %v", v)
		}
	}
	if count != 3 {
		t.Errorf("All() yielded %d elements, want 3 (snapshot of original queue)", count)
	}
	if pq.Len() != 6 {
		t.Errorf("Len() after mutating inside All() = %d, want 6", pq.Len())
	}
}

// TestSingleElement covers single-element edge cases for every operation.
func TestSingleElement(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	pq.Insert(&PqTest{value: "only", priority: 7})

	if pq.Len() != 1 || pq.IsEmpty() {
		t.Fatalf("Len() = %d, IsEmpty() = %v, want 1, false", pq.Len(), pq.IsEmpty())
	}
	if v := pq.Peek(); v == nil || v.value != "only" {
		t.Errorf("Peek() = %v, want only", v)
	}

	// Update the single element's priority.
	if !pq.UpdatePriority(0, &PqTest{value: "only", priority: 42}) {
		t.Error("UpdatePriority(0) = false, want true")
	}
	if v := pq.Peek(); v == nil || v.priority != 42 {
		t.Errorf("Peek() after UpdatePriority = %v, want priority 42", v)
	}

	// Delete the single element.
	if err := pq.Delete(0); err != nil {
		t.Errorf("Delete(0) error = %v, want nil", err)
	}
	if !pq.IsEmpty() {
		t.Error("IsEmpty() = false after deleting the only element")
	}

	// Insert again, iterate, then pop.
	pq.Insert(&PqTest{value: "again", priority: 1})
	count := 0
	for v := range pq.All() {
		if v.value != "again" {
			t.Errorf("All() yielded %v, want again", v)
		}
		count++
	}
	if count != 1 {
		t.Errorf("All() yielded %d elements, want 1", count)
	}
	if v := pq.Pop(); v == nil || v.value != "again" {
		t.Errorf("Pop() = %v, want again", v)
	}
	if v := pq.Pop(); v != nil {
		t.Errorf("Pop() on drained queue = %v, want nil", v)
	}
}

// TestDuplicatePriorities verifies that duplicate priorities are stored
// independently and all come out.
func TestDuplicatePriorities(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	for i := 0; i < 5; i++ {
		pq.Insert(&PqTest{value: fmt.Sprintf("dup%d", i), priority: 3})
	}
	pq.Insert(&PqTest{value: "low", priority: 1})
	pq.Insert(&PqTest{value: "high", priority: 9})

	if pq.Len() != 7 {
		t.Fatalf("Len() = %d, want 7", pq.Len())
	}
	if v := pq.Pop(); v == nil || v.priority != 1 {
		t.Errorf("first Pop() = %v, want priority 1", v)
	}
	for i := 0; i < 5; i++ {
		if v := pq.Pop(); v == nil || v.priority != 3 {
			t.Errorf("Pop() #%d = %v, want priority 3", i, v)
		}
	}
	if v := pq.Pop(); v == nil || v.priority != 9 {
		t.Errorf("last Pop() = %v, want priority 9", v)
	}
	if !pq.IsEmpty() {
		t.Error("queue should be empty after popping all duplicates")
	}
}

// TestTruncateThenReuse covers Truncate on an empty queue and reuse after
// repeated truncation.
func TestTruncateThenReuse(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	pq.Truncate() // Truncate on an empty queue must be a no-op.
	if !pq.IsEmpty() {
		t.Error("IsEmpty() = false after Truncate on empty queue")
	}

	for round := 0; round < 3; round++ {
		for i := 0; i < 10; i++ {
			pq.Insert(&PqTest{value: "x", priority: i})
		}
		pq.Truncate()
		if pq.Len() != 0 || !pq.IsEmpty() {
			t.Fatalf("round %d: Len() = %d after Truncate, want 0", round, pq.Len())
		}
		if v := pq.Pop(); v != nil {
			t.Fatalf("round %d: Pop() after Truncate = %v, want nil", round, v)
		}
	}

	pq.Insert(&PqTest{value: "survivor", priority: 1})
	if v := pq.Peek(); v == nil || v.value != "survivor" {
		t.Errorf("Peek() after reuse = %v, want survivor", v)
	}
}
