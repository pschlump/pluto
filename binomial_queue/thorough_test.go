package binomial_queue

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"reflect"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Structural invariants
// -------------------------------------------------------------------------------------------------------

// checkInvariants verifies the binomial-queue structure: the forest is
// in strictly increasing order of degree, every tree is a true binomial
// tree of its degree (min-heap-ordered per cmp, children of degrees
// 0..k-1, exactly 2^k nodes), and the total node count matches Length.
// Call it after every structural change.
func checkInvariants[T any](t *testing.T, q *BinomialQueue[T], cmp func(a, b T) int) {
	t.Helper()
	if q == nil {
		return
	}
	total := 0
	prevDegree := -1
	for _, tr := range q.trees {
		degree := len(tr.children)
		if degree <= prevDegree {
			t.Errorf("forest invariant violated: tree of degree %d follows degree %d (not strictly increasing)",
				degree, prevDegree)
		}
		prevDegree = degree
		total += checkBinomialTree(t, tr, cmp)
	}
	if total != q.Length() {
		t.Errorf("Length mismatch: Length()=%d but the forest has %d nodes", q.Length(), total)
	}
	if total == 0 && !q.IsEmpty() {
		t.Errorf("IsEmpty() is false but the queue has no nodes")
	}
	if total > 0 && q.IsEmpty() {
		t.Errorf("IsEmpty() is true but the queue has %d nodes", total)
	}
}

// checkBinomialTree verifies that n roots a true binomial tree of degree
// len(n.children): child i has degree i (so the tree has 2^degree
// nodes), and every child sorts at or after its parent per cmp.  It
// returns the number of nodes in the tree.
func checkBinomialTree[T any](t *testing.T, n *bqNode[T], cmp func(a, b T) int) int {
	t.Helper()
	degree := len(n.children)
	count := 1
	for i, c := range n.children {
		if len(c.children) != i {
			t.Errorf("binomial tree invariant violated: child %d has degree %d, expected %d",
				i, len(c.children), i)
		}
		if cmp(c.value, n.value) < 0 {
			t.Errorf("heap-order invariant violated: child %v sorts before its parent %v", c.value, n.value)
		}
		count += checkBinomialTree(t, c, cmp)
	}
	if count != 1<<degree {
		t.Errorf("degree-%d binomial tree has %d nodes, expected %d", degree, count, 1<<degree)
	}
	return count
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestRandomizedModel runs 800 mixed Insert/DeleteMin/Peek/Merge steps
// against a multiset reference model with a fixed seed, verifying that
// DeleteMin returns the true minimum each time and that the structural
// invariants hold after every step.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	q := NewBinomialQueue[int]()
	model := map[int]int{} // value -> count

	modelMin := func() int {
		min := -1
		for k := range model {
			if min == -1 || k < min {
				min = k
			}
		}
		return min
	}
	modelCount := func() int {
		n := 0
		for _, c := range model {
			n += c
		}
		return n
	}
	removeFromModel := func(v int) {
		model[v]--
		if model[v] == 0 {
			delete(model, v)
		}
	}

	verify := func(step int) {
		t.Helper()
		checkInvariants(t, q, Compare[int])
		if q.Len() != modelCount() {
			t.Fatalf("step %d: Len %d, model has %d", step, q.Len(), modelCount())
		}
		// Multiset equality.
		seen := map[int]int{}
		for _, v := range q.All() {
			seen[v]++
		}
		if !reflect.DeepEqual(seen, model) {
			t.Fatalf("step %d: queue multiset %v != model %v", step, seen, model)
		}
		// Peek/FindMin must agree with the model minimum.
		if len(model) == 0 {
			if _, found := q.Peek(); found {
				t.Fatalf("step %d: Peek on empty queue reported true", step)
			}
		} else if v, found := q.FindMin(); !found || v != modelMin() {
			t.Fatalf("step %d: FindMin = (%v, %v), model min %d", step, v, found, modelMin())
		}
	}

	const keySpace = 100
	for step := range 800 {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert
			v := rng.Intn(keySpace)
			q.Insert(v)
			model[v]++
		case 4, 5, 6: // DeleteMin must return the true minimum
			got, found := q.DeleteMin()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: DeleteMin on empty reported true", step)
				}
			} else {
				min := modelMin()
				if !found || got != min {
					t.Fatalf("step %d: DeleteMin = (%v, %v), model min %d", step, got, found, min)
				}
				removeFromModel(min)
			}
		case 7, 8: // Merge a freshly built queue in
			other := NewBinomialQueue[int]()
			m := rng.Intn(20)
			for range m {
				v := rng.Intn(keySpace)
				other.Insert(v)
				model[v]++
			}
			q.Merge(other)
			if !other.IsEmpty() || other.Len() != 0 {
				t.Fatalf("step %d: other not empty after Merge (Len %d)", step, other.Len())
			}
		case 9: // Peek only
		}
		verify(step)
	}

	// Drain: the rest comes out in non-decreasing order and matches the model.
	prev := -1
	for {
		v, found := q.DeleteMin()
		if !found {
			break
		}
		if v < prev {
			t.Fatalf("drain: %d after %d, not in ascending order", v, prev)
		}
		prev = v
		removeFromModel(v)
		checkInvariants(t, q, Compare[int])
	}
	if len(model) != 0 {
		t.Errorf("model not drained: %v", model)
	}
}

// TestMergeThenDeleteMin is a focused Merge check: two queues built from
// interleaved value ranges must drain as one sorted stream.
func TestMergeThenDeleteMin(t *testing.T) {
	a := NewBinomialQueue[int]()
	b := NewBinomialQueue[int]()
	for i := range 50 {
		a.Insert(2 * i) // evens
		checkInvariants(t, a, Compare[int])
	}
	for i := range 50 {
		b.Insert(2*i + 1) // odds
		checkInvariants(t, b, Compare[int])
	}
	a.Merge(b)
	checkInvariants(t, a, Compare[int])
	if a.Len() != 100 {
		t.Fatalf("Expected length 100 after Merge, got %d", a.Len())
	}
	for i := range 100 {
		v, found := a.DeleteMin()
		if !found || v != i {
			t.Fatalf("DeleteMin = (%v, %v), expected %d", v, found, i)
		}
	}
}
