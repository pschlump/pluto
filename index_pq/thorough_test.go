package index_pq

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// checkInvariants verifies the structural invariants of the queue:
//   - the heap property: the value at every heap position orders no
//     earlier than the value at its parent position;
//   - the inverse position map: qp[pq[i]] == i for every heap position;
//   - qp[k] == -1 exactly when index k is absent, and otherwise it
//     points back at k (pq[qp[k]] == k);
//   - length equals the number of indices with qp[k] != -1, and Len
//     agrees.
func checkInvariants[T any](t *testing.T, q *IndexPQ[T], cmp func(a, b T) int) {
	t.Helper()

	// Heap property over pq via cmp.
	for i := 1; i < q.length; i++ {
		parent := (i - 1) / 2
		if cmp(q.vals[q.pq[i]], q.vals[q.pq[parent]]) < 0 {
			t.Fatalf("heap invariant violated: vals[pq[%d]]=%v sorts before its parent vals[pq[%d]]=%v",
				i, q.vals[q.pq[i]], parent, q.vals[q.pq[parent]])
		}
	}

	// Every heap position holds a valid index, and qp is its exact inverse.
	for i := 0; i < q.length; i++ {
		k := q.pq[i]
		if k < 0 || k >= q.n {
			t.Fatalf("pq[%d] = %d is out of the index space 0..%d", i, k, q.n-1)
		}
		if q.qp[k] != i {
			t.Fatalf("qp[pq[%d]] = qp[%d] = %d, expected %d", i, k, q.qp[k], i)
		}
	}

	// qp[k] == -1 ⟺ k is absent; count of present indices == length.
	count := 0
	for k := 0; k < q.n; k++ {
		if q.qp[k] == -1 {
			if q.Contains(k) {
				t.Fatalf("qp[%d] is -1 but Contains(%d) reports present", k, k)
			}
			continue
		}
		count++
		if q.qp[k] < 0 || q.qp[k] >= q.length {
			t.Fatalf("qp[%d] = %d is not a heap position (length %d)", k, q.qp[k], q.length)
		}
		if q.pq[q.qp[k]] != k {
			t.Fatalf("pq[qp[%d]] = pq[%d] = %d, expected %d", k, q.qp[k], q.pq[q.qp[k]], k)
		}
		if !q.Contains(k) {
			t.Fatalf("qp[%d] = %d but Contains(%d) reports absent", k, q.qp[k], k)
		}
	}
	if count != q.length {
		t.Fatalf("length is %d but %d indices have qp[k] != -1", q.length, count)
	}
	if q.Len() != q.length {
		t.Fatalf("Len() = %d does not match internal length %d", q.Len(), q.length)
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestIndexPQRandomizedModel runs 800 mixed operations (Insert, Change,
// Delete, Pop, Peek, Value/Contains) against a map reference model with
// a fixed seed, verifying the structural invariants, the length, and —
// every few steps — that All drains exactly the model in priority order.
func TestIndexPQRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64        // index space 0..63
	const valSpace = 40 // small value space so ties are common

	q := NewIndexPQ[int](n)
	model := make(map[int]int) // index -> value the queue should hold

	modelMin := func() (minK, minV int, ok bool) {
		for k, v := range model {
			if !ok || v < minV {
				minK, minV, ok = k, v, true
			}
		}
		return
	}

	verify := func(step int) {
		if q.Len() != len(model) {
			t.Fatalf("step %d: Len()=%d, model has %d indices", step, q.Len(), len(model))
		}
		checkInvariants(t, q, Compare[int])

		// All must yield exactly the model, in non-decreasing value order.
		seen := make(map[int]int, len(model))
		prev := -1
		for k, v := range q.All() {
			if prev >= 0 && v < prev {
				t.Fatalf("step %d: All yielded %d after %d — not in priority order", step, v, prev)
			}
			prev = v
			seen[k] = v
		}
		if len(seen) != len(model) {
			t.Fatalf("step %d: All yielded %d pairs, model has %d", step, len(seen), len(model))
		}
		for k, v := range model {
			if sv, ok := seen[k]; !ok || sv != v {
				t.Fatalf("step %d: All pair for %d = (%d, %v), model says %d", step, k, sv, ok, v)
			}
		}

		// Peek must match a model minimum (ties allowed between equal values).
		if len(model) == 0 {
			if _, _, found := q.Peek(); found {
				t.Fatalf("step %d: Peek on empty queue reported true", step)
			}
		} else {
			_, minV, _ := modelMin()
			k, v, found := q.Peek()
			if !found || v != minV || model[k] != v {
				t.Fatalf("step %d: Peek = (%d, %d, %v), model min value %d", step, k, v, found, minV)
			}
		}
	}

	for step := range 800 {
		// Occasionally probe just outside the index space.
		k := rng.Intn(n+2) - 1 // -1..n
		v := rng.Intn(valSpace)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert (in-range always true; present indices replace)
			got := q.Insert(k, v)
			if k < 0 || k >= n {
				if got {
					t.Fatalf("step %d: Insert(%d) reported true for an out-of-range index", step, k)
				}
				break
			}
			if !got {
				t.Fatalf("step %d: Insert(%d) reported false for an in-range index", step, k)
			}
			model[k] = v
		case 4, 5: // Delete
			_, present := model[k]
			if got := q.Delete(k); got != present {
				t.Fatalf("step %d: Delete(%d)=%v, model said present=%v", step, k, got, present)
			}
			delete(model, k)
		case 6, 7: // Change
			_, present := model[k]
			if got := q.Change(k, v); got != present {
				t.Fatalf("step %d: Change(%d)=%v, model said present=%v", step, k, got, present)
			}
			if present {
				model[k] = v
			}
		case 8: // Pop
			k, v, found := q.Pop()
			if len(model) == 0 {
				if found {
					t.Fatalf("step %d: Pop on empty queue reported true", step)
				}
				break
			}
			_, minV, _ := modelMin()
			mv, present := model[k]
			if !found || !present || mv != v || v != minV {
				t.Fatalf("step %d: Pop = (%d, %d, %v), model min value %d", step, k, v, found, minV)
			}
			delete(model, k)
		case 9: // Value / Contains
			mv, present := model[k]
			if got := q.Contains(k); got != present {
				t.Fatalf("step %d: Contains(%d)=%v, model said present=%v", step, k, got, present)
			}
			gv, found := q.Value(k)
			if found != present || (present && gv != mv) {
				t.Fatalf("step %d: Value(%d)=(%d, %v), model said (%d, %v)", step, k, gv, found, mv, present)
			}
		}
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)
}

// TestIndexPQFullDrainAndRefill stresses the queue at its capacity: fill
// all n slots, change every key, delete half, pop the rest, refill.
func TestIndexPQFullDrainAndRefill(t *testing.T) {
	const n = 128
	q := NewIndexPQ[int](n)

	for k := 0; k < n; k++ {
		if !q.Insert(k, (k*37)%n) { // shuffled values, with ties
			t.Fatalf("Insert(%d) into a non-full queue reported false", k)
		}
		checkInvariants(t, q, Compare[int])
	}
	if q.Len() != n {
		t.Fatalf("Expected Len %d on a full queue, got %d", n, q.Len())
	}

	// Change every key to its own index value (strictly increasing).
	for k := 0; k < n; k++ {
		if !q.Change(k, k) {
			t.Fatalf("Change(%d) on a full queue reported false", k)
		}
	}
	checkInvariants(t, q, Compare[int])

	// Delete the even indices.
	for k := 0; k < n; k += 2 {
		if !q.Delete(k) {
			t.Fatalf("Delete(%d) reported false", k)
		}
		checkInvariants(t, q, Compare[int])
	}

	// The odd indices pop in ascending order (value == index now).
	for want := 1; want < n; want += 2 {
		k, v, found := q.Pop()
		if !found || k != want || v != want {
			t.Fatalf("Pop = (%d, %d, %v), expected (%d, %d, true)", k, v, found, want, want)
		}
	}
	if !q.IsEmpty() {
		t.Fatalf("Expected empty queue after draining.")
	}

	// Refill after a full drain.
	for k := 0; k < n; k++ {
		q.Insert(k, n-k)
	}
	checkInvariants(t, q, Compare[int])
	if k, v, found := q.Peek(); !found || k != n-1 || v != 1 {
		t.Errorf("Peek after refill = (%d, %d, %v), expected (%d, 1, true)", k, v, found, n-1)
	}
}
