package index_pq_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
)

// checkInvariants verifies the structural invariants of the queue (heap
// property, inverse position map, length).  It reads the internals
// WITHOUT taking the lock — single-goroutine tests only (or call it only
// once all concurrent work is known to be quiescent).
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
			continue
		}
		count++
		if q.qp[k] < 0 || q.qp[k] >= q.length {
			t.Fatalf("qp[%d] = %d is not a heap position (length %d)", k, q.qp[k], q.length)
		}
		if q.pq[q.qp[k]] != k {
			t.Fatalf("pq[qp[%d]] = pq[%d] = %d, expected %d", k, q.qp[k], q.pq[q.qp[k]], k)
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

// TestIndexPQRandomizedModel runs 800 mixed operations against a map
// reference model with a fixed seed.  It is single-goroutine, so
// checkInvariants may read the internals directly.
func TestIndexPQRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const n = 64        // index space 0..63
	const valSpace = 40 // small value space so ties are common

	q := NewIndexPQ[int](n)
	model := make(map[int]int) // index -> value the queue should hold

	modelMin := func() (minV int, ok bool) {
		for _, v := range model {
			if !ok || v < minV {
				minV, ok = v, true
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
			minV, _ := modelMin()
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
			minV, _ := modelMin()
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

// -------------------------------------------------------------------------------------------------------
// Thread-safety: run under -race
// -------------------------------------------------------------------------------------------------------

// TestConcurrentInsertChangePop hammers one shared queue from many
// goroutines: writers Insert and Change striped (disjoint) indices while
// an observer iterates the snapshot All and reads Peek/Len, then all
// goroutines Pop concurrently — every inserted index must come out
// exactly once.
func TestConcurrentInsertChangePop(t *testing.T) {
	const n = 1000
	const workers = 8

	q := NewIndexPQ[int](n)

	var stop atomic.Bool
	var observerWG sync.WaitGroup
	observerWG.Add(1)
	go func() {
		defer observerWG.Done()
		for !stop.Load() {
			// The snapshot is a valid queue: each pass yields
			// non-decreasing values regardless of concurrent mutation.
			prev := -1
			first := true
			for _, v := range q.All() {
				if !first && v < prev {
					t.Errorf("All yielded %d after %d — snapshot not in priority order", v, prev)
					return
				}
				first = false
				prev = v
			}
			q.Peek()
			q.Len()
			q.IsEmpty()
		}
	}()

	// Writers on disjoint stripes: no two goroutines touch the same index.
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := w; k < n; k += workers {
				q.Insert(k, (k*37)%n)
				q.Change(k, (k*91)%n) // always present: own stripe
				q.Contains(k)
				q.Value(k)
			}
		}(w)
	}
	wg.Wait()

	if q.Len() != n {
		t.Fatalf("Expected Len %d after concurrent fills, got %d", n, q.Len())
	}
	checkInvariants(t, q, Compare[int]) // writers are done: quiescent

	// Concurrent pops: every index comes out exactly once.
	var mu sync.Mutex
	seen := make(map[int]bool, n)
	total := 0
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				k, _, found := q.Pop()
				if !found {
					return
				}
				mu.Lock()
				if seen[k] {
					t.Errorf("index %d popped twice", k)
				}
				seen[k] = true
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	stop.Store(true)
	observerWG.Wait()

	if total != n {
		t.Errorf("Expected %d popped indices, got %d", n, total)
	}
	if q.Len() != 0 || !q.IsEmpty() {
		t.Errorf("Expected drained queue, got Len %d", q.Len())
	}
}

// TestConcurrentCompound exercises the Lock + Nl* compound surface:
// atomic decrease-key-if-greater clamps from many goroutines must leave
// every value at or below every clamp threshold ever applied, with no
// torn read-then-write.
func TestConcurrentCompound(t *testing.T) {
	const n = 200
	const workers = 8

	q := NewIndexPQ[int](n)
	for k := range n {
		q.Insert(k, 1000+k)
	}

	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 100 {
				k := (w*100 + i) % n
				threshold := i
				q.Lock()
				if q.NlContains(k) {
					if v, found := q.NlValue(k); found && v > threshold {
						q.NlChange(k, threshold)
					}
				}
				q.Unlock()
			}
		}(w)
	}
	wg.Wait()

	// Every threshold 0..99 was applied to every index, so every value is
	// at most 99.
	if q.Len() != n {
		t.Fatalf("Expected Len %d after compound clamps, got %d", n, q.Len())
	}
	for k, v := range q.All() {
		if v > 99 {
			t.Errorf("index %d has value %d after clamps to 0..99 — torn compound", k, v)
		}
	}
	checkInvariants(t, q, Compare[int])

	// Concurrent Lock + NlPop: each pop is atomic, no index pops twice.
	var mu sync.Mutex
	seen := make(map[int]bool, n)
	total := 0
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				q.Lock()
				if q.NlLen() == 0 {
					q.Unlock()
					return
				}
				k, _, _ := q.NlPop()
				q.Unlock()
				mu.Lock()
				if seen[k] {
					t.Errorf("index %d popped twice", k)
				}
				seen[k] = true
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if total != n {
		t.Errorf("Expected %d popped indices via NlPop, got %d", n, total)
	}
}
