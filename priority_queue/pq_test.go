package priority_queue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// PqTest is the test element type.  Ordering is supplied to the queue
// as a plain function (cmpPqTest below).
type PqTest struct {
	value    string
	priority int
}

// cmpPqTest orders PqTest by its priority field.
func cmpPqTest(a, b PqTest) int {
	return a.priority - b.priority
}

// newTestPQ builds a priority queue of PqTest ordered by priority,
// pre-loaded with the given items.
func newTestPQ(items ...PqTest) *PriorityQueue[PqTest] {
	pq := NewPriorityQueueFunc(cmpPqTest)
	for _, it := range items {
		pq.Insert(it)
	}
	return pq
}

// pqPriorities collects the queue's priorities (via All, which is in
// priority order) as a slice.
func pqPriorities(pq *PriorityQueue[PqTest]) []int {
	var got []int
	for v := range pq.All() {
		got = append(got, v.priority)
	}
	return got
}

// checkPQ verifies length, contents-as-multiset, and that the multiset
// matches the model.
func checkPQ(t *testing.T, pq *PriorityQueue[PqTest], want map[int]int, context string) {
	t.Helper()
	total := 0
	for _, c := range want {
		total += c
	}
	if pq.Len() != total {
		t.Fatalf("%s: Len()=%d, want %d", context, pq.Len(), total)
	}
	got := map[int]int{}
	for _, p := range pqPriorities(pq) {
		got[p]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: contents mismatch: got %v, want %v", context, got, want)
	}
}

func TestNewEmptyQueue(t *testing.T) {
	pq := NewPriorityQueueFunc(cmpPqTest)

	if !pq.IsEmpty() {
		t.Errorf("Expected empty queue.")
	}
	if pq.Len() != 0 || pq.Length() != 0 {
		t.Errorf("Expected length 0, got %d/%d", pq.Len(), pq.Length())
	}
	if _, found := pq.Peek(); found {
		t.Errorf("Expected Peek on empty queue to report false.")
	}
	if _, found := pq.Pop(); found {
		t.Errorf("Expected Pop on empty queue to report false.")
	}
	if _, _, found := pq.Search(PqTest{priority: 1}); found {
		t.Errorf("Expected Search on empty queue to report false.")
	}
	if _, found := pq.Delete(0); found {
		t.Errorf("Expected Delete on empty queue to report false.")
	}
	if pq.UpdatePriority(0, PqTest{priority: 1}) {
		t.Errorf("Expected UpdatePriority on empty queue to report false.")
	}
	n := 0
	for range pq.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from All on empty queue, got %d", n)
	}
}

func TestInsertPeekPop(t *testing.T) {
	pq := newTestPQ()

	// With equal input the minimum is stable across peeks.
	pq.Insert(PqTest{value: "a", priority: 5})
	if v, found := pq.Peek(); !found || v.priority != 5 {
		t.Errorf("Peek = (%v, %v), want priority 5", v, found)
	}
	if pq.Len() != 1 {
		t.Errorf("Peek must not remove; length %d", pq.Len())
	}

	pq.Insert(PqTest{value: "b", priority: 2})
	pq.Insert(PqTest{value: "c", priority: 8})
	pq.Insert(PqTest{value: "d", priority: 1})

	// Pop always returns the minimum.
	for _, want := range []int{1, 2, 5, 8} {
		v, found := pq.Pop()
		if !found {
			t.Fatalf("Pop: unexpectedly empty (want priority %d).", want)
		}
		if v.priority != want {
			t.Errorf("Pop priority = %d, want %d", v.priority, want)
		}
	}
	if _, found := pq.Pop(); found {
		t.Errorf("Expected Pop on drained queue to report false.")
	}
	if !pq.IsEmpty() {
		t.Errorf("Expected empty queue after draining.")
	}
}

func TestSearch(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 5},
		PqTest{value: "b", priority: 2},
		PqTest{value: "c", priority: 8},
	)

	// A probe needs only the fields the comparison reads.
	if v, pos, found := pq.Search(PqTest{priority: 8}); !found || pos < 0 || pos >= pq.Len() {
		t.Errorf("Search(8) = (%v, %d, %v)", v, pos, found)
	} else if v.value != "c" {
		t.Errorf("Search(8) returned %q, want c", v.value)
	}

	if v, pos, found := pq.Search(PqTest{priority: 42}); found || pos != -1 {
		t.Errorf("Search(42) = (%v, %d, %v), want (zero, -1, false)", v, pos, found)
	}
}

func TestUpdatePriority(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 10},
		PqTest{value: "b", priority: 20},
		PqTest{value: "c", priority: 30},
	)

	// Lower "c" to the minimum.
	_, pos, found := pq.Search(PqTest{priority: 30})
	if !found {
		t.Fatalf("Expected to find priority 30.")
	}
	if !pq.UpdatePriority(pos, PqTest{value: "c", priority: 1}) {
		t.Fatalf("UpdatePriority(%d) reported false on a valid position.", pos)
	}
	if v, found := pq.Peek(); !found || v.priority != 1 {
		t.Errorf("After lowering: Peek = (%v, %v), want priority 1", v, found)
	}

	// Raise the minimum to the maximum.
	if !pq.UpdatePriority(0, PqTest{value: "c", priority: 99}) {
		t.Fatalf("UpdatePriority(0) reported false.")
	}
	if v, found := pq.Peek(); !found || v.priority != 10 {
		t.Errorf("After raising: Peek = (%v, %v), want priority 10", v, found)
	}

	// Out of range reports false and changes nothing.
	for _, bad := range []int{-1, pq.Len()} {
		if pq.UpdatePriority(bad, PqTest{priority: 1}) {
			t.Errorf("UpdatePriority(%d) reported true, want false", bad)
		}
	}
	if pq.Len() != 3 {
		t.Errorf("Invalid UpdatePriority changed length to %d", pq.Len())
	}
}

func TestDelete(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 1},
		PqTest{value: "b", priority: 2},
		PqTest{value: "c", priority: 3},
	)

	// Delete the middle element by position.
	_, pos, _ := pq.Search(PqTest{priority: 2})
	if v, found := pq.Delete(pos); !found || v.priority != 2 {
		t.Errorf("Delete(%d) = (%v, %v), want priority 2", pos, v, found)
	}
	if pq.Len() != 2 {
		t.Errorf("Expected length 2 after delete, got %d", pq.Len())
	}
	if v, found := pq.Peek(); !found || v.priority != 1 {
		t.Errorf("Peek after delete = (%v, %v), want priority 1", v, found)
	}

	// Out of range reports false.
	for _, bad := range []int{-1, pq.Len(), 100} {
		if _, found := pq.Delete(bad); found {
			t.Errorf("Delete(%d) reported true, want false", bad)
		}
	}
	if pq.Len() != 2 {
		t.Errorf("Invalid Delete changed length to %d", pq.Len())
	}

	// The drain is still in order.
	for _, want := range []int{1, 3} {
		if v, found := pq.Pop(); !found || v.priority != want {
			t.Errorf("Pop = (%v, %v), want priority %d", v, found, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 1},
		PqTest{value: "b", priority: 2},
	)
	pq.Truncate()

	if !pq.IsEmpty() || pq.Len() != 0 {
		t.Errorf("Expected empty queue after Truncate.")
	}
	if _, found := pq.Peek(); found {
		t.Errorf("Expected Peek after Truncate to report false.")
	}

	// Reusable after the drain.
	pq.Insert(PqTest{value: "z", priority: 9})
	if v, found := pq.Peek(); !found || v.priority != 9 {
		t.Errorf("Peek after Truncate+Insert = (%v, %v), want 9", v, found)
	}

	// Truncating an already-empty queue is fine.
	pq.Truncate()
	pq.Truncate()
	if !pq.IsEmpty() {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

func TestAllIterator(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 5},
		PqTest{value: "b", priority: 2},
		PqTest{value: "c", priority: 8},
		PqTest{value: "d", priority: 1},
	)

	// All yields in priority order, minimum first.
	var got []int
	for v := range pq.All() {
		got = append(got, v.priority)
	}
	if expect := []int{1, 2, 5, 8}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All got %v, want %v", got, expect)
	}

	// The iteration is non-destructive.
	if pq.Len() != 4 {
		t.Errorf("All must not drain; length %d", pq.Len())
	}

	// Early break stops iteration.
	n := 0
	for range pq.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}
}

// TestAllIteratorSnapshot verifies All operates on a snapshot taken when
// it is called: later modifications — even truncating the whole queue —
// are not observed, and mutating from inside the loop is safe.
func TestAllIteratorSnapshot(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 5},
		PqTest{value: "b", priority: 2},
		PqTest{value: "c", priority: 8},
	)

	all := pq.All()

	pq.Truncate() // the iterator above must not observe this

	var got []int
	for v := range all {
		got = append(got, v.priority)
	}
	if expect := []int{2, 5, 8}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All after Truncate got %v, want %v", got, expect)
	}

	// Mutating from inside the loop is safe.
	pq = newTestPQ(
		PqTest{value: "a", priority: 3},
		PqTest{value: "b", priority: 1},
	)
	visited := 0
	for v := range pq.All() {
		visited++
		pq.Pop()
		_ = v
	}
	if visited != 2 {
		t.Errorf("Expected 2 visits while popping during iteration, got %d", visited)
	}
	if !pq.IsEmpty() {
		t.Errorf("Expected empty queue after popping during iteration.")
	}
}

func TestAllEmptyAndSingle(t *testing.T) {
	empty := NewPriorityQueueFunc(cmpPqTest)
	n := 0
	for range empty.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from All on empty queue, got %d", n)
	}

	single := newTestPQ(PqTest{value: "only", priority: 7})
	var got []int
	for v := range single.All() {
		got = append(got, v.priority)
	}
	if expect := []int{7}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All on single-element queue got %v, want %v", got, expect)
	}
	if single.Len() != 1 {
		t.Errorf("All must not drain; length %d", single.Len())
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructors, zero value, nil queue
// -------------------------------------------------------------------------------------------------------

// TestNewPriorityQueueOrdered verifies the constructor for naturally
// ordered element types.
func TestNewPriorityQueueOrdered(t *testing.T) {
	pq := NewPriorityQueue[int]()
	for _, v := range []int{42, 7, 13} {
		pq.Insert(v)
	}
	if v, found := pq.Peek(); !found || v != 7 {
		t.Errorf("Peek = (%v, %v), want 7", v, found)
	}
	var got []int
	for v := range pq.All() {
		got = append(got, v)
	}
	if expect := []int{7, 13, 42}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All got %v, want %v", got, expect)
	}
}

// TestNewPriorityQueueFunc verifies the func constructor, including a
// reversed (max-first) ordering.
func TestNewPriorityQueueFunc(t *testing.T) {
	// Highest priority first: reversed comparison.
	pq := NewPriorityQueueFunc(func(a, b int) int { return b - a })
	for _, v := range []int{1, 5, 3} {
		pq.Insert(v)
	}
	var got []int
	for v := range pq.All() {
		got = append(got, v)
	}
	if expect := []int{5, 3, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Max-first All got %v, want %v", got, expect)
	}
}

// TestNewPriorityQueueFuncNil verifies that a nil comparison function is
// rejected at construction time, not on first use.
func TestNewPriorityQueueFuncNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected NewPriorityQueueFunc(nil) to panic.")
		}
	}()
	NewPriorityQueueFunc[PqTest](nil)
}

// TestZeroValueQueue verifies that the zero value of PriorityQueue
// behaves as an empty queue for every read and that Insert fails loudly.
func TestZeroValueQueue(t *testing.T) {
	var pq PriorityQueue[PqTest]

	if !pq.IsEmpty() {
		t.Errorf("Expected zero value queue to be empty.")
	}
	if pq.Len() != 0 || pq.Length() != 0 {
		t.Errorf("Expected zero value queue to have length 0.")
	}
	if _, found := pq.Peek(); found {
		t.Errorf("Expected Peek on zero value queue to report false.")
	}
	if _, found := pq.Pop(); found {
		t.Errorf("Expected Pop on zero value queue to report false.")
	}
	if _, _, found := pq.Search(PqTest{priority: 1}); found {
		t.Errorf("Expected Search on zero value queue to report false.")
	}
	if _, found := pq.Delete(0); found {
		t.Errorf("Expected Delete on zero value queue to report false.")
	}
	if pq.UpdatePriority(0, PqTest{priority: 1}) {
		t.Errorf("Expected UpdatePriority on zero value queue to report false.")
	}
	pq.Truncate() // no-op, must not panic
	n := 0
	for range pq.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from All on zero value queue.")
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("Expected Insert on zero value queue to panic.")
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewPriorityQueue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		pq.Insert(PqTest{priority: 1})
	}()
}

// TestNilQueueTolerated verifies that every operation except Insert
// treats a nil queue as an empty queue, and that Insert panics with a
// message naming the insert family.
func TestNilQueueTolerated(t *testing.T) {
	var pq *PriorityQueue[PqTest]

	if !pq.IsEmpty() {
		t.Errorf("Expected nil queue to be empty.")
	}
	if pq.Len() != 0 || pq.Length() != 0 {
		t.Errorf("Expected nil queue to have length 0.")
	}
	if _, found := pq.Peek(); found {
		t.Errorf("Expected Peek on nil queue to report false.")
	}
	if _, found := pq.Pop(); found {
		t.Errorf("Expected Pop on nil queue to report false.")
	}
	if _, _, found := pq.Search(PqTest{priority: 1}); found {
		t.Errorf("Expected Search on nil queue to report false.")
	}
	if _, found := pq.Delete(0); found {
		t.Errorf("Expected Delete on nil queue to report false.")
	}
	if pq.UpdatePriority(0, PqTest{priority: 1}) {
		t.Errorf("Expected UpdatePriority on nil queue to report false.")
	}
	pq.Lock()     // no-op
	pq.Truncate() // no-op
	pq.Unlock()   // no-op
	n := 0
	for range pq.All() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from All on nil queue.")
	}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert on nil queue to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		pq.Insert(PqTest{priority: 1})
	}()
}

// TestLockUnlockNoop verifies the compatibility shims exist.
func TestLockUnlockNoop(t *testing.T) {
	pq := newTestPQ(PqTest{value: "a", priority: 1})
	pq.Lock()
	pq.Insert(PqTest{value: "b", priority: 2}) // the shims are no-ops, so Insert works inside the "critical section"
	pq.Unlock()
	if pq.Len() != 2 {
		t.Errorf("Expected length 2, got %d", pq.Len())
	}
}

func TestSingleElement(t *testing.T) {
	pq := newTestPQ(PqTest{value: "only", priority: 7})

	if pq.IsEmpty() || pq.Len() != 1 {
		t.Errorf("Expected single-element queue, length %d", pq.Len())
	}
	if v, found := pq.Peek(); !found || v.value != "only" {
		t.Errorf("Peek = (%v, %v)", v, found)
	}
	if _, pos, found := pq.Search(PqTest{priority: 7}); !found || pos != 0 {
		t.Errorf("Search = (%d, %v)", pos, found)
	}
	if _, found := pq.Pop(); !found {
		t.Errorf("Expected Pop on single-element queue to report true.")
	}
	if !pq.IsEmpty() {
		t.Errorf("Expected empty queue after popping the only element.")
	}
}

// TestDuplicatePriorities verifies duplicates coexist and pop in
// non-decreasing priority order.
func TestDuplicatePriorities(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 5},
		PqTest{value: "b", priority: 5},
		PqTest{value: "c", priority: 1},
		PqTest{value: "d", priority: 5},
	)

	prev := -1
	var got []int
	for !pq.IsEmpty() {
		v, found := pq.Pop()
		if !found {
			t.Fatalf("Pop: unexpectedly empty.")
		}
		if v.priority < prev {
			t.Fatalf("Pop out of order: %d after %d", v.priority, prev)
		}
		prev = v.priority
		got = append(got, v.priority)
	}
	if expect := []int{1, 5, 5, 5}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain got %v, want %v", got, expect)
	}
}

// -------------------------------------------------------------------------------------------------------
// Property test against a multiset reference model
// -------------------------------------------------------------------------------------------------------

// TestPQRandomOpsAgainstReference hammers Insert/Pop/Peek/Search/
// UpdatePriority/Delete in random order (with duplicate priorities) and
// cross-checks every operation against a multiset reference model.
func TestPQRandomOpsAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(1234, 5678))

	for trial := range 100 {
		pq := NewPriorityQueueFunc(cmpPqTest)
		want := make(map[int]int)

		for op := range 200 {
			ctx := fmt.Sprintf("trial %d op %d", trial, op)
			n := pq.Len()
			choice := rng.IntN(100)
			switch {
			case choice < 35 || n == 0:
				// Insert
				p := rng.IntN(50)
				pq.Insert(PqTest{value: fmt.Sprintf("v%d", op), priority: p})
				want[p]++
			case choice < 55:
				// Pop
				got, found := pq.Pop()
				mn, ok := refMin(want)
				if !ok {
					t.Fatalf("%s: Pop on non-empty queue reported false", ctx)
				}
				if !found || got.priority != mn {
					t.Fatalf("%s: Pop got (%v, %v), want priority %d", ctx, got, found, mn)
				}
				refDel(want, mn)
			case choice < 60:
				// Peek
				got, found := pq.Peek()
				mn, _ := refMin(want)
				if !found || got.priority != mn {
					t.Fatalf("%s: Peek got (%v, %v), want %d", ctx, got, found, mn)
				}
				if pq.Len() != n {
					t.Fatalf("%s: Peek changed length %d -> %d", ctx, n, pq.Len())
				}
			case choice < 70:
				// Search for an existing or missing priority
				probe := PqTest{priority: rng.IntN(55)}
				rv, pos, found := pq.Search(probe)
				if _, exists := want[probe.priority]; exists {
					if !found || pos < 0 || pos >= pq.Len() {
						t.Fatalf("%s: Search(%d) failed: rv=%v pos=%d found=%v", ctx, probe.priority, rv, pos, found)
					}
				} else {
					if found || pos != -1 {
						t.Fatalf("%s: Search(%d) should miss: rv=%v pos=%d found=%v", ctx, probe.priority, rv, pos, found)
					}
				}
			case choice < 85:
				// UpdatePriority at a random valid position
				pos := rng.IntN(n)
				old, _ := pq.h.GetValue(pos)
				np := rng.IntN(50)
				if !pq.UpdatePriority(pos, PqTest{value: "u", priority: np}) {
					t.Fatalf("%s: UpdatePriority(%d) returned false on valid position", ctx, pos)
				}
				refDel(want, old.priority)
				want[np]++
			default:
				// Delete at a random valid position
				pos := rng.IntN(n)
				old, _ := pq.h.GetValue(pos)
				if _, found := pq.Delete(pos); !found {
					t.Fatalf("%s: Delete(%d) returned false on valid position", ctx, pos)
				}
				refDel(want, old.priority)
			}
			checkPQ(t, pq, want, ctx)
		}

		// Drain in non-decreasing priority order.
		prev := -1
		for pq.Len() > 0 {
			got, found := pq.Pop()
			if !found {
				t.Fatalf("trial %d drain: Pop reported false", trial)
			}
			if got.priority < prev {
				t.Fatalf("trial %d drain: Pop out of order: %d after %d", trial, got.priority, prev)
			}
			prev = got.priority
		}
		if !pq.IsEmpty() {
			t.Fatalf("trial %d: IsEmpty false after drain", trial)
		}
	}
}

func refMin(want map[int]int) (int, bool) {
	keys := make([]int, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return 0, false
	}
	for i := 1; i < len(keys); i++ { // tiny insertion sort; maps are unordered
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys[0], true
}

func refDel(want map[int]int, v int) {
	want[v]--
	if want[v] <= 0 {
		delete(want, v)
	}
}

// TestPQOutOfRange verifies UpdatePriority/Delete reject invalid
// positions without modifying the queue.
func TestPQOutOfRange(t *testing.T) {
	pq := newTestPQ(PqTest{value: "a", priority: 1}, PqTest{value: "b", priority: 2})
	for _, pos := range []int{-1, 2, 100} {
		if pq.UpdatePriority(pos, PqTest{priority: 9}) {
			t.Errorf("UpdatePriority(%d) returned true, want false", pos)
		}
		if _, found := pq.Delete(pos); found {
			t.Errorf("Delete(%d) reported true, want false", pos)
		}
		if pq.Len() != 2 {
			t.Fatalf("invalid op changed length to %d", pq.Len())
		}
	}
	if got, found := pq.Peek(); !found || got.priority != 1 {
		t.Fatalf("Peek after invalid ops got (%v, %v), want priority 1", got, found)
	}
}

// TestPQUpdatePriorityBothDirections verifies that raising and lowering a
// priority both restore correct Pop order.
func TestPQUpdatePriorityBothDirections(t *testing.T) {
	pq := newTestPQ(
		PqTest{value: "a", priority: 10},
		PqTest{value: "b", priority: 20},
		PqTest{value: "c", priority: 30},
		PqTest{value: "d", priority: 40},
	)

	// Find "d" (priority 40) and make it the minimum.
	_, pos, found := pq.Search(PqTest{priority: 40})
	if !found {
		t.Fatalf("Search(40) failed.")
	}
	if !pq.UpdatePriority(pos, PqTest{value: "d", priority: 5}) {
		t.Fatalf("UpdatePriority(%d) failed", pos)
	}
	if got, found := pq.Peek(); !found || got.priority != 5 {
		t.Fatalf("after lowering: Peek got (%v, %v), want priority 5", got, found)
	}

	// Find "a" (priority 10) and make it the maximum.
	_, pos, found = pq.Search(PqTest{priority: 10})
	if !found {
		t.Fatalf("Search(10) failed.")
	}
	if !pq.UpdatePriority(pos, PqTest{value: "a", priority: 90}) {
		t.Fatalf("UpdatePriority(%d) failed", pos)
	}
	if got, found := pq.Peek(); !found || got.priority != 5 {
		t.Fatalf("after raising another: Peek got (%v, %v), want priority 5", got, found)
	}

	// The drain is fully ordered.
	var got []int
	for !pq.IsEmpty() {
		v, _ := pq.Pop()
		got = append(got, v.priority)
	}
	if expect := []int{5, 20, 30, 90}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Drain got %v, want %v", got, expect)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// PqJSONItem is the JSON test element type; its fields are exported so
// the encoding/json package can round-trip them.
type PqJSONItem struct {
	Value    string
	Priority int
}

// cmpPqJSONItem orders PqJSONItem by its Priority field.
func cmpPqJSONItem(a, b PqJSONItem) int {
	return a.Priority - b.Priority
}

// pqAllInts collects an int queue's elements via All (priority order).
func pqAllInts(pq *PriorityQueue[int]) []int {
	var got []int
	for v := range pq.All() {
		got = append(got, v)
	}
	return got
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output in priority order, minimum first — the
	// insertion order does not survive the heap.
	pq := NewPriorityQueue[int]()
	for _, v := range []int{3, 1, 2} {
		pq.Insert(v)
	}
	b, err := json.Marshal(pq)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != "[1,2,3]" {
		t.Errorf("Expected [1,2,3], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	items := NewPriorityQueueFunc(cmpPqJSONItem)
	items.Insert(PqJSONItem{Value: "b", Priority: 2})
	items.Insert(PqJSONItem{Value: "a", Priority: 1})
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"Value":"a","Priority":1},{"Value":"b","Priority":2}]` {
		t.Errorf("Unexpected struct encoding: (%s, %v)", b, err)
	}

	// An empty queue encodes as [].
	if b, err := json.Marshal(NewPriorityQueue[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty queue, got (%s, %v)", b, err)
	}

	// A zero-value queue is a tolerated read: [].
	var zero PriorityQueue[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value queue, got (%s, %v)", b, err)
	}

	// A direct call on a nil queue encodes as []; json.Marshal on a nil
	// *PriorityQueue never reaches the method — the json package writes
	// null for nil pointers itself.
	var nilPQ *PriorityQueue[int]
	if b, err := nilPQ.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-queue call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilPQ); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil queue, got (%s, %v)", b, err)
	}

	// Element-level marshaling errors propagate unchanged.
	bad := NewPriorityQueueFunc(func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a queue of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// The decoded elements replace the contents; they come back out in
	// priority order regardless of the array order.
	pq := NewPriorityQueue[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), pq); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, expect := pqAllInts(pq), []int{1, 2, 3}; !reflect.DeepEqual(got, expect) {
		t.Errorf("All got %v, want %v", got, expect)
	}

	// Round-trip: marshal, unmarshal into a fresh queue, same order.
	b, err := json.Marshal(pq)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	rt := NewPriorityQueue[int]()
	if err := json.Unmarshal(b, rt); err != nil {
		t.Fatalf("json.Unmarshal round-trip: %v", err)
	}
	if got, expect := pqAllInts(rt), []int{1, 2, 3}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Round-trip All got %v, want %v", got, expect)
	}

	// Unmarshaling replaces, not appends.
	if err := json.Unmarshal([]byte("[9,7]"), pq); err != nil {
		t.Fatalf("json.Unmarshal replace: %v", err)
	}
	if got, expect := pqAllInts(pq), []int{7, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After replace All got %v, want %v", got, expect)
	}

	// The comparison function is kept: the queue stays usable.
	pq.Insert(5)
	if got, expect := pqAllInts(pq), []int{5, 7, 9}; !reflect.DeepEqual(got, expect) {
		t.Errorf("After Insert All got %v, want %v", got, expect)
	}

	// null and [] clear the queue.
	for _, data := range []string{"null", "[]"} {
		if err := json.Unmarshal([]byte(data), pq); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if !pq.IsEmpty() {
			t.Errorf("Expected empty queue after unmarshaling %s.", data)
		}
	}

	// Struct elements round-trip through their normal JSON encoding.
	items := NewPriorityQueueFunc(cmpPqJSONItem)
	if err := json.Unmarshal([]byte(`[{"Value":"b","Priority":2},{"Value":"a","Priority":1}]`), items); err != nil {
		t.Fatalf("json.Unmarshal struct: %v", err)
	}
	if v, found := items.Peek(); !found || v.Value != "a" || v.Priority != 1 {
		t.Errorf("Peek after struct unmarshal = (%v, %v), want {a 1}", v, found)
	}
}

// TestUnmarshalJSONDecodeError verifies that a decode error is returned
// with the queue untouched.
func TestUnmarshalJSONDecodeError(t *testing.T) {
	pq := NewPriorityQueue[int]()
	for _, v := range []int{1, 2, 3} {
		pq.Insert(v)
	}
	for _, data := range []string{
		"not json",  // malformed
		`{"a":1}`,   // not an array
		`[1,"x",3]`, // wrong element type
		`[1,2`,      // truncated
	} {
		if err := json.Unmarshal([]byte(data), pq); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", data)
		}
	}
	if got, expect := pqAllInts(pq), []int{1, 2, 3}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Queue changed by decode errors: All got %v, want %v", got, expect)
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value queue panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero PriorityQueue[int]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value queue to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value queue.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewPriorityQueue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte("[1]"))
	}()

	var nilPQ *PriorityQueue[int]
	if err := nilPQ.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil queue to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil queue.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil or zero-value queue") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilPQ.UnmarshalJSON([]byte("[1]"))
	}()
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------------------------------------

const benchmarkPQSize = 4096

func BenchmarkInsert(b *testing.B) {
	pq := NewPriorityQueueFunc(cmpPqTest)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if pq.Len() >= benchmarkPQSize {
			pq.Truncate()
		}
		pq.Insert(PqTest{value: "v", priority: i})
	}
}

func BenchmarkPop(b *testing.B) {
	pq := NewPriorityQueueFunc(cmpPqTest)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if pq.IsEmpty() {
			for j := range benchmarkPQSize {
				pq.Insert(PqTest{value: "v", priority: j})
			}
		}
		pq.Pop()
	}
}

func BenchmarkPeek(b *testing.B) {
	pq := NewPriorityQueueFunc(cmpPqTest)
	for j := range benchmarkPQSize {
		pq.Insert(PqTest{value: "v", priority: j})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Peek()
	}
}
