package priority_queue

import (
	"fmt"
	"testing"
)

// TestZeroValuePanics verifies the documented contract that the zero value
// is not usable: any operation on it must panic (nil internal heap).
func TestZeroValuePanics(t *testing.T) {
	var pq PriorityQueue[PqTest]
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Len() on zero-value PriorityQueue should panic (zero value is not usable)")
			}
		}()
		pq.Len()
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Insert() on zero-value PriorityQueue should panic (zero value is not usable)")
			}
		}()
		pq.Insert(&PqTest{value: "x", priority: 1})
	}()
}

// TestLockUnlockNoOps verifies that Lock/Unlock exist and are harmless
// no-ops, so code written against priority_queue_ts compiles and runs
// unchanged against this package.
func TestLockUnlockNoOps(t *testing.T) {
	pq := newTestPQ(PqTest{value: "a", priority: 1})
	pq.Lock()
	pq.Unlock()
	pq.Lock()
	pq.Lock()
	pq.Unlock()
	pq.Unlock()
	if pq.Len() != 1 {
		t.Errorf("Len() after Lock/Unlock = %d, want 1", pq.Len())
	}
}

// TestSingleElement covers the size-1 boundary: insert one element, peek,
// delete the only position, and verify empty-state behavior afterwards.
func TestSingleElement(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	pq.Insert(&PqTest{value: "only", priority: 7})

	if pq.Len() != 1 || pq.IsEmpty() {
		t.Fatalf("after one Insert: Len()=%d IsEmpty()=%v, want 1/false", pq.Len(), pq.IsEmpty())
	}
	if top := pq.Peek(); top == nil || top.value != "only" {
		t.Fatalf("Peek() = %v, want only", top)
	}

	// Search must find the only element at position 0.
	rv, pos, err := pq.Search(&PqTest{priority: 7})
	if err != nil || rv == nil || pos != 0 {
		t.Fatalf("Search() = %v, %d, %v; want element at pos 0, nil error", rv, pos, err)
	}

	// UpdatePriority on the only element.
	if !pq.UpdatePriority(0, &PqTest{value: "only", priority: 42}) {
		t.Fatal("UpdatePriority(0) = false, want true")
	}
	if top := pq.Peek(); top == nil || top.priority != 42 {
		t.Fatalf("Peek() after UpdatePriority = %v, want priority 42", top)
	}

	// Delete the only element.
	if err := pq.Delete(0); err != nil {
		t.Fatalf("Delete(0) error = %v, want nil", err)
	}
	if !pq.IsEmpty() {
		t.Fatal("queue should be empty after deleting the only element")
	}
	if v := pq.Pop(); v != nil {
		t.Errorf("Pop() on empty queue = %v, want nil", v)
	}

	// Insert again and pop it: single-element push/pop round trip.
	pq.Insert(&PqTest{value: "again", priority: 3})
	if got := pq.Pop(); got == nil || got.value != "again" {
		t.Fatalf("Pop() = %v, want again", got)
	}
	if !pq.IsEmpty() {
		t.Error("queue should be empty after popping the only element")
	}
}

// TestDuplicatePriorities verifies that duplicate priorities are stored
// independently (no replacement) and all copies come back out.
func TestDuplicatePriorities(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 5},
		PqTest{value: "b", priority: 5},
		PqTest{value: "c", priority: 5},
		PqTest{value: "d", priority: 1},
		PqTest{value: "e", priority: 1},
	)
	if pq.Len() != 5 {
		t.Fatalf("Len() = %d, want 5 (duplicates must not be coalesced)", pq.Len())
	}

	count1, count5 := 0, 0
	for pq.Len() > 0 {
		got := pq.Pop()
		switch got.priority {
		case 1:
			count1++
		case 5:
			count5++
			// All priority-5 items must come after all priority-1 items.
			if count1 != 2 {
				t.Errorf("Pop() returned priority 5 before all priority 1 items were drained")
			}
		default:
			t.Errorf("Pop() returned unexpected priority %d", got.priority)
		}
	}
	if count1 != 2 || count5 != 3 {
		t.Errorf("drained %d items of priority 1 and %d of priority 5, want 2 and 3", count1, count5)
	}
}

// TestAllIteratorSnapshot verifies that All() iterates a snapshot: elements
// inserted or removed during iteration do not change the yielded sequence.
func TestAllIteratorSnapshot(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 1},
		PqTest{value: "b", priority: 2},
		PqTest{value: "c", priority: 3},
	)

	var got []int
	first := true
	for item := range pq.All() {
		got = append(got, item.priority)
		if first {
			first = false
			// Mutate the live queue mid-iteration.
			pq.Insert(&PqTest{value: "z", priority: 0})
			pq.Pop()
		}
	}
	want := []int{1, 2, 3}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("All() with mid-iteration mutation yielded %v, want snapshot %v", got, want)
	}

	// The live queue now holds b, c (a was popped; z was inserted and the
	// pop removed the min, which is a).
	if pq.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", pq.Len())
	}

	// Early break after two elements must not consume the rest.
	count := 0
	for range pq.All() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break: iterated %d elements, want 2", count)
	}
}

// TestAllBreakBeforeFirst verifies that breaking out of All() before the
// first yield (a zero-trip loop body) leaves the queue intact.
func TestAllEmptyAndSingle(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()
	for range pq.All() {
		t.Error("All() on empty queue should yield nothing")
	}

	pq.Insert(&PqTest{value: "solo", priority: 9})
	var got []string
	for item := range pq.All() {
		got = append(got, item.value)
	}
	if len(got) != 1 || got[0] != "solo" {
		t.Errorf("All() on single-element queue yielded %v, want [solo]", got)
	}
}

// TestNlCompatibilityAPI exercises every Nl-prefixed method and verifies it
// behaves identically to its plain counterpart. These methods exist so code
// written against priority_queue_ts compiles unchanged.
func TestNlCompatibilityAPI(t *testing.T) {
	pq := NewPriorityQueue[PqTest]()

	if !pq.NlIsEmpty() {
		t.Error("NlIsEmpty() on new queue = false, want true")
	}
	if pq.NlLen() != 0 {
		t.Errorf("NlLen() on new queue = %d, want 0", pq.NlLen())
	}
	if v := pq.NlPeek(); v != nil {
		t.Errorf("NlPeek() on empty queue = %v, want nil", v)
	}
	if v := pq.NlPop(); v != nil {
		t.Errorf("NlPop() on empty queue = %v, want nil", v)
	}

	pq.NlInsert(&PqTest{value: "banana", priority: 3})
	pq.NlInsert(&PqTest{value: "apple", priority: 2})
	pq.NlInsert(&PqTest{value: "pear", priority: 4})

	if pq.NlLen() != 3 {
		t.Fatalf("NlLen() = %d, want 3", pq.NlLen())
	}
	if pq.NlIsEmpty() {
		t.Error("NlIsEmpty() = true, want false")
	}
	if top := pq.NlPeek(); top == nil || top.value != "apple" {
		t.Fatalf("NlPeek() = %v, want apple", top)
	}

	// NlSearch hit and miss.
	rv, pos, err := pq.NlSearch(&PqTest{priority: 3})
	if err != nil || rv == nil || rv.value != "banana" || pos < 0 || pos >= pq.NlLen() {
		t.Fatalf("NlSearch(3) = %v, %d, %v; want banana at valid pos, nil error", rv, pos, err)
	}
	if rv, _, err = pq.NlSearch(&PqTest{priority: 99}); err == nil || rv != nil {
		t.Errorf("NlSearch(99) = %v, %v; want nil, non-nil error", rv, err)
	}

	// NlUpdatePriority: valid and invalid positions.
	if !pq.NlUpdatePriority(pos, &PqTest{value: "banana", priority: 10}) {
		t.Fatal("NlUpdatePriority(valid) = false, want true")
	}
	if top := pq.NlPeek(); top == nil || top.value != "apple" {
		t.Fatalf("NlPeek() after NlUpdatePriority = %v, want apple", top)
	}
	if pq.NlUpdatePriority(-1, &PqTest{priority: 1}) {
		t.Error("NlUpdatePriority(-1) = true, want false")
	}
	if pq.NlUpdatePriority(pq.NlLen(), &PqTest{priority: 1}) {
		t.Error("NlUpdatePriority(Len) = true, want false")
	}

	// NlDelete: valid and invalid positions.
	if err := pq.NlDelete(-1); err == nil {
		t.Error("NlDelete(-1) returned nil error, want non-nil")
	}
	if err := pq.NlDelete(pq.NlLen()); err == nil {
		t.Error("NlDelete(Len) returned nil error, want non-nil")
	}
	if err := pq.NlDelete(0); err != nil {
		t.Fatalf("NlDelete(0) error = %v, want nil", err)
	}
	if pq.NlLen() != 2 {
		t.Fatalf("NlLen() after NlDelete = %d, want 2", pq.NlLen())
	}

	// NlDelete(0) removed the heap root, which is the minimum ("apple", 2).
	// NlPop drains the rest in priority order.
	want := []int{4, 10}
	for i, w := range want {
		got := pq.NlPop()
		if got == nil || got.priority != w {
			t.Fatalf("NlPop() #%d = %v, want priority %d", i, got, w)
		}
	}
	if !pq.NlIsEmpty() {
		t.Error("NlIsEmpty() = false after draining, want true")
	}
}

// TestSearchReturnsLiveElement documents that Search returns a live *T
// alias into the queue's storage (the package returns pointers, not copies).
func TestSearchReturnsLiveElement(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 1},
		PqTest{value: "b", priority: 2},
	)
	rv, _, err := pq.Search(&PqTest{priority: 2})
	if err != nil || rv == nil {
		t.Fatalf("Search() error = %v", err)
	}
	// Mutating through the returned pointer is visible via a later Pop of
	// that same element (heap ordering aside, the stored value changes).
	rv.value = "mutated"
	found := false
	for pq.Len() > 0 {
		if pq.Pop().value == "mutated" {
			found = true
		}
	}
	if !found {
		t.Error("mutation through Search result not visible; expected a live *T alias")
	}
}
