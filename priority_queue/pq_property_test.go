// Copyright (C) Philip Schlump, 2012-2021. BSD 3 Clause Licensed.

package priority_queue

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// pqContents collects the queue's elements (in priority order via All) as a
// multiset of priorities.
func pqContents(pq *PriorityQueue[PqTest]) map[int]int {
	got := make(map[int]int, pq.Len())
	for v := range pq.All() {
		got[v.priority]++
	}
	return got
}

func checkPQ(t *testing.T, pq *PriorityQueue[PqTest], want map[int]int, context string) {
	t.Helper()
	total := 0
	for _, c := range want {
		total += c
	}
	if pq.Len() != total {
		t.Fatalf("%s: Len()=%d, want %d", context, pq.Len(), total)
	}
	got := pqContents(pq)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s: contents mismatch: got %v, want %v", context, got, want)
	}
}

func pqRefMin(want map[int]int) (int, bool) {
	keys := make([]int, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return 0, false
	}
	sort.Ints(keys)
	return keys[0], true
}

func pqRefDel(want map[int]int, v int) {
	want[v]--
	if want[v] <= 0 {
		delete(want, v)
	}
}

// TestPQRandomOpsAgainstReference hammers Insert/Pop/Peek/Search/
// UpdatePriority/Delete in random order (with duplicate priorities) and
// cross-checks every operation against a multiset reference model.
func TestPQRandomOpsAgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))

	for trial := 0; trial < 100; trial++ {
		pq := NewPriorityQueue[PqTest]()
		want := make(map[int]int)

		for op := 0; op < 200; op++ {
			ctx := fmt.Sprintf("trial %d op %d", trial, op)
			n := pq.Len()
			choice := rng.Intn(100)
			switch {
			case choice < 35 || n == 0:
				// Insert
				p := rng.Intn(50)
				pq.Insert(&PqTest{value: fmt.Sprintf("v%d", op), priority: p})
				want[p]++
			case choice < 55:
				// Pop
				got := pq.Pop()
				mn, ok := refMinForPQ(t, want, ctx)
				if !ok {
					t.Fatalf("%s: Pop on non-empty queue returned nil", ctx)
				}
				if got == nil || got.priority != mn {
					t.Fatalf("%s: Pop got %v, want priority %d", ctx, got, mn)
				}
				pqRefDel(want, mn)
			case choice < 60:
				// Peek
				got := pq.Peek()
				mn, _ := pqRefMin(want)
				if got == nil || got.priority != mn {
					t.Fatalf("%s: Peek got %v, want %d", ctx, got, mn)
				}
				if pq.Len() != n {
					t.Fatalf("%s: Peek changed length %d -> %d", ctx, n, pq.Len())
				}
			case choice < 70:
				// Search for an existing or missing priority
				probe := &PqTest{priority: rng.Intn(55)}
				rv, pos, err := pq.Search(probe)
				if _, exists := want[probe.priority]; exists {
					if err != nil || rv == nil || pos < 0 || pos >= pq.Len() {
						t.Fatalf("%s: Search(%d) failed: rv=%v pos=%d err=%v", ctx, probe.priority, rv, pos, err)
					}
				} else {
					if err == nil || rv != nil || pos != -1 {
						t.Fatalf("%s: Search(%d) should miss: rv=%v pos=%d err=%v", ctx, probe.priority, rv, pos, err)
					}
				}
			case choice < 85:
				// UpdatePriority at a random valid position
				pos := rng.Intn(n)
				old := pq.h.GetValue(pos).priority
				np := rng.Intn(50)
				if !pq.UpdatePriority(pos, &PqTest{value: "u", priority: np}) {
					t.Fatalf("%s: UpdatePriority(%d) returned false on valid position", ctx, pos)
				}
				pqRefDel(want, old)
				want[np]++
			default:
				// Delete at a random valid position
				pos := rng.Intn(n)
				old := pq.h.GetValue(pos).priority
				if err := pq.Delete(pos); err != nil {
					t.Fatalf("%s: Delete(%d) returned error %v", ctx, pos, err)
				}
				pqRefDel(want, old)
			}
			checkPQ(t, pq, want, ctx)
		}

		// Drain in non-decreasing priority order.
		prev := -1
		for pq.Len() > 0 {
			got := pq.Pop()
			if got == nil {
				t.Fatalf("trial %d drain: Pop returned nil", trial)
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

func refMinForPQ(t *testing.T, want map[int]int, ctx string) (int, bool) {
	t.Helper()
	return pqRefMin(want)
}

// TestPQOutOfRange verifies UpdatePriority/Delete reject invalid positions
// without modifying the queue.
func TestPQOutOfRange(t *testing.T) {
	pq := newTestPQ(PqTest{value: "a", priority: 1}, PqTest{value: "b", priority: 2})
	for _, pos := range []int{-1, 2, 100} {
		if pq.UpdatePriority(pos, &PqTest{priority: 9}) {
			t.Errorf("UpdatePriority(%d) returned true, want false", pos)
		}
		if err := pq.Delete(pos); err == nil {
			t.Errorf("Delete(%d) returned nil error, want non-nil", pos)
		}
		if pq.Len() != 2 {
			t.Fatalf("invalid op changed length to %d", pq.Len())
		}
	}
	if got := pq.Peek(); got == nil || got.priority != 1 {
		t.Fatalf("Peek after invalid ops got %v, want priority 1", got)
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
	_, pos, err := pq.Search(&PqTest{priority: 40})
	if err != nil {
		t.Fatalf("Search(40): %v", err)
	}
	if !pq.UpdatePriority(pos, &PqTest{value: "d", priority: 5}) {
		t.Fatalf("UpdatePriority(%d) failed", pos)
	}
	if got := pq.Peek(); got == nil || got.priority != 5 {
		t.Fatalf("after lowering: Peek got %v, want priority 5", got)
	}

	// Find "a" (priority 10) and make it the maximum.
	_, pos, err = pq.Search(&PqTest{priority: 10})
	if err != nil {
		t.Fatalf("Search(10): %v", err)
	}
	if !pq.UpdatePriority(pos, &PqTest{value: "a", priority: 99}) {
		t.Fatalf("UpdatePriority(%d) failed", pos)
	}

	wantOrder := []int{5, 20, 30, 99}
	for i, want := range wantOrder {
		got := pq.Pop()
		if got == nil || got.priority != want {
			t.Fatalf("%d.th Pop got %v, want priority %d", i, got, want)
		}
	}
}

// TestPQDeleteEveryPosition deletes each position in turn and verifies the
// remaining drain order.
func TestPQDeleteEveryPosition(t *testing.T) {
	const size = 20
	for pos := 0; pos < size; pos++ {
		pq := NewPriorityQueue[PqTest]()
		// Insert in reverse so positions differ from priority order.
		for i := size; i >= 1; i-- {
			pq.Insert(&PqTest{value: fmt.Sprintf("v%d", i), priority: i})
		}
		removed := pq.h.GetValue(pos).priority
		if err := pq.Delete(pos); err != nil {
			t.Fatalf("Delete(%d): %v", pos, err)
		}
		if pq.Len() != size-1 {
			t.Fatalf("Delete(%d): Len()=%d, want %d", pos, pq.Len(), size-1)
		}
		prev := 0
		count := 0
		for pq.Len() > 0 {
			got := pq.Pop()
			if got.priority == removed {
				t.Fatalf("Delete(%d): removed priority %d still present", pos, removed)
			}
			if got.priority < prev {
				t.Fatalf("Delete(%d): Pop out of order: %d after %d", pos, got.priority, prev)
			}
			prev = got.priority
			count++
		}
		if count != size-1 {
			t.Fatalf("Delete(%d): drained %d elements, want %d", pos, count, size-1)
		}
	}
}

// TestPQAllIsNonDestructive verifies All yields sorted order and leaves the
// queue untouched, even when iterating twice.
func TestPQAllIsNonDestructive(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	pq := NewPriorityQueue[PqTest]()
	want := make(map[int]int)
	for i := 0; i < 100; i++ {
		p := rng.Intn(30)
		pq.Insert(&PqTest{value: "x", priority: p})
		want[p]++
	}

	for round := 0; round < 2; round++ {
		prev := -1
		count := 0
		for v := range pq.All() {
			if v.priority < prev {
				t.Fatalf("round %d: All out of order: %d after %d", round, v.priority, prev)
			}
			prev = v.priority
			count++
		}
		if count != 100 {
			t.Fatalf("round %d: All yielded %d, want 100", round, count)
		}
		if pq.Len() != 100 {
			t.Fatalf("round %d: All changed Len() to %d", round, pq.Len())
		}
	}
	checkPQ(t, pq, want, "after All")
}
