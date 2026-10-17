/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stable_pq

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/priority_queue"
)

// TestRandomizedModel mirrors 4000 random queue operations against a
// stable-sorted-slice oracle: every Pop/Peek must return the minimum of
// the model by (key, insertion sequence) — the exact order a stable
// sort of the inserted elements produces.  Keys are drawn from a small
// range so ties are the common case, not the exception.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	pq := NewStablePQFunc(byPrio)

	type entry struct {
		prio int
		tag  int // insertion sequence, assigned by the model
	}
	var model []entry
	nextTag := 0

	modelMin := func() (entry, bool) {
		if len(model) == 0 {
			return entry{}, false
		}
		best := model[0]
		for _, e := range model[1:] {
			if e.prio < best.prio || (e.prio == best.prio && e.tag < best.tag) {
				best = e
			}
		}
		return best, true
	}

	for step := 0; step < 4000; step++ {
		switch rng.IntN(3) {
		case 0, 1: // insert twice as often as pop, so the queue grows
			prio := rng.IntN(5) // tiny key range: heavy ties
			model = append(model, entry{prio: prio, tag: nextTag})
			pq.Insert(task{prio: prio, tag: itoa(nextTag)})
			nextTag++
		case 2:
			want, found := modelMin()
			got, ok := pq.Pop()
			if !found {
				if ok {
					t.Fatalf("step %d: Pop on empty queue returned (%v, true)", step, got)
				}
				continue
			}
			if !ok {
				t.Fatalf("step %d: Pop returned not-found, model has %d elements", step, len(model))
			}
			if got.prio != want.prio || got.tag != itoa(want.tag) {
				t.Fatalf("step %d: Pop = (%d, %s), model wants (%d, %d)", step, got.prio, got.tag, want.prio, want.tag)
			}
			model = slices.DeleteFunc(model, func(e entry) bool {
				return e.prio == want.prio && e.tag == want.tag
			})
		}

		if pq.Len() != len(model) {
			t.Fatalf("step %d: Len = %d, model has %d", step, pq.Len(), len(model))
		}

		// Every 37 steps cross-check the full state: All must equal the
		// stable-sorted model, exactly.
		if step%37 == 0 {
			sorted := slices.Clone(model)
			slices.SortStableFunc(sorted, func(a, b entry) int {
				if a.prio != b.prio {
					return a.prio - b.prio
				}
				return a.tag - b.tag
			})
			var got []task
			for x := range pq.All() {
				got = append(got, x)
			}
			if len(got) != len(sorted) {
				t.Fatalf("step %d: All yielded %d elements, model has %d", step, len(got), len(sorted))
			}
			for i := range sorted {
				if got[i].prio != sorted[i].prio || got[i].tag != itoa(sorted[i].tag) {
					t.Fatalf("step %d: All[%d] = (%d,%s), model wants (%d,%d)", step, i, got[i].prio, got[i].tag, sorted[i].prio, sorted[i].tag)
				}
			}
		}
	}

	// Drain the remainder and compare against the stable-sorted model.
	slices.SortStableFunc(model, func(a, b entry) int {
		if a.prio != b.prio {
			return a.prio - b.prio
		}
		return a.tag - b.tag
	})
	for i, want := range model {
		got, ok := pq.Pop()
		if !ok {
			t.Fatalf("drain: queue empty at %d, model has %d left", i, len(model)-i)
		}
		if got.prio != want.prio || got.tag != itoa(want.tag) {
			t.Fatalf("drain[%d]: Pop = (%d,%s), model wants (%d,%d)", i, got.prio, got.tag, want.prio, want.tag)
		}
	}
	if !pq.IsEmpty() {
		t.Fatalf("drain: queue still has %d elements after the model emptied", pq.Len())
	}
}

// TestInterleavedTieGroups drives the queue through many rounds of
// mixing inserts and partial drains — the pattern where an unstable
// heap's internal reordering would eventually surface — over a medium
// key range.
func TestInterleavedTieGroups(t *testing.T) {
	// seqTask's comparator orders by prio only; the unique numeric seq
	// is satellite data tracking insertion order (the stability
	// witness).  The model orders by (prio, seq) — numerically, not
	// lexicographically through a string form.
	type seqTask struct {
		prio int
		seq  int
	}
	prioOnly := func(a, b seqTask) int { return a.prio - b.prio }

	rng := rand.New(rand.NewPCG(7, 42))
	pq := NewStablePQFunc(prioOnly)
	var expected []seqTask

	for round := 0; round < 200; round++ {
		for i := 0; i < 25; i++ {
			x := seqTask{prio: rng.IntN(20), seq: round*25 + i}
			pq.Insert(x)
			expected = append(expected, x)
		}
		for i := 0; i < 10; i++ {
			got, ok := pq.Pop()
			if !ok {
				t.Fatalf("round %d: unexpected empty pop", round)
			}
			// The element popped must be the minimum of the current
			// expected set by (prio, seq); find and remove it.
			bi := 0
			for j, e := range expected {
				if e.prio < expected[bi].prio || (e.prio == expected[bi].prio && e.seq < expected[bi].seq) {
					bi = j
				}
			}
			if got.prio != expected[bi].prio || got.seq != expected[bi].seq {
				t.Fatalf("round %d pop %d: got (%d,%d), want (%d,%d)", round, i, got.prio, got.seq, expected[bi].prio, expected[bi].seq)
			}
			expected = slices.Delete(expected, bi, bi+1)
		}
	}

	slices.SortStableFunc(expected, func(a, b seqTask) int {
		if a.prio != b.prio {
			return a.prio - b.prio
		}
		return a.seq - b.seq
	})
	for i, want := range expected {
		got, _ := pq.Pop()
		if got.prio != want.prio || got.seq != want.seq {
			t.Fatalf("final drain[%d]: got (%d,%d), want (%d,%d)", i, got.prio, got.seq, want.prio, want.seq)
		}
	}
	if !pq.IsEmpty() {
		t.Fatalf("final drain: %d elements left", pq.Len())
	}
}

func stringCompare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// TestAgainstPlainPriorityQueue cross-checks the VALUE multiset against
// pluto's plain priority queue over the same operations: the set of
// popped keys (ignoring order within ties — the plain queue does not
// promise stability) must be identical at every step.
func TestAgainstPlainPriorityQueue(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 5))
	spq := NewStablePQ[int]()
	plain := priority_queue.NewPriorityQueue[int]()

	for step := 0; step < 3000; step++ {
		switch rng.IntN(2) {
		case 0:
			v := rng.IntN(50)
			spq.Insert(v)
			plain.Insert(v)
		case 1:
			sv, sok := spq.Pop()
			pv, pok := plain.Pop()
			if sok != pok {
				t.Fatalf("step %d: found mismatch %v vs %v", step, sok, pok)
			}
			if sok && sv != pv {
				// Both are minima of the same multiset, so the values
				// must be equal even if the queues disagree on WHICH
				// equal element to hand back (invisible for bare ints).
				t.Fatalf("step %d: Pop values differ: %d vs %d", step, sv, pv)
			}
		}
		if spq.Len() != plain.Len() {
			t.Fatalf("step %d: Len diverged: %d vs %d", step, spq.Len(), plain.Len())
		}
	}
}

// TestOrderedConstructor orders strings with the built-in operators via
// NewStablePQ — the cmp.Ordered path with a tie-heavy string multiset.
func TestOrderedConstructor(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 99))
	pq := NewStablePQ[string]()
	var inserted []string
	words := []string{"pear", "apple", "fig", "kiwi", "apple", "fig"}
	for i := 0; i < 300; i++ {
		w := words[rng.IntN(len(words))]
		inserted = append(inserted, w)
		pq.Insert(w)
	}
	slices.SortStableFunc(inserted, func(a, b string) int { return stringCompare(a, b) })
	for i, want := range inserted {
		got, ok := pq.Pop()
		if !ok || got != want {
			t.Fatalf("ordered drain[%d]: got (%q,%v), want %q", i, got, ok, want)
		}
	}
}
