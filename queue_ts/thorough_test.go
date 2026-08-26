package queue_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: single-element and duplicate edge cases, value-copy
// semantics, iterators after a partial drain, and a fixed-seed randomized
// FIFO property test cross-checked against a slice reference model.

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// contents returns the current contents of the queue, head to tail.  It
// always returns a non-nil slice so empty-queue comparisons work with
// reflect.DeepEqual.
func contents[T any](q *Queue[T]) []T {
	got := []T{}
	for _, v := range q.All() {
		got = append(got, v)
	}
	return got
}

// TestSingleElement exercises every operation on a one-element queue.
func TestSingleElement(t *testing.T) {
	var q Queue[string]

	q.Push("only")
	if q.IsEmpty() || q.Length() != 1 {
		t.Errorf("Expected single-element queue, length %d", q.Length())
	}
	if v, err := q.Peek(); err != nil || v != "only" {
		t.Errorf("Peek = (%q, %v)", v, err)
	}
	if v, err := q.Dequeue(); err != nil || v != "only" {
		t.Errorf("Dequeue = (%q, %v)", v, err)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeueing the only element.")
	}
	if q.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
}

// TestDuplicateValues verifies that duplicates coexist and dequeue in
// FIFO order.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]
	for _, v := range []int{5, 5, 5, 7} {
		q.Push(v)
	}
	for i, want := range []int{5, 5, 5, 7} {
		if v, err := q.Dequeue(); err != nil || v != want {
			t.Errorf("Dequeue step %d = (%v, %v), expected %d", i, v, err, want)
		}
	}
}

// TestDequeueReturnsValue verifies that the dequeued element is an
// independent value: mutating a dequeued struct does not affect the queue
// (or vice versa), because elements are stored and returned by value.
func TestDequeueReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var q Queue[item]
	q.Push(item{S: "a", N: 1})
	q.Push(item{S: "b", N: 2})

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	v.N = 99 // mutate the returned value

	if rest := contents(&q); !reflect.DeepEqual(rest, []item{{S: "b", N: 2}}) {
		t.Errorf("Mutating a dequeued value affected the queue: %v", rest)
	}
	if next, err := q.Dequeue(); err != nil || next.N != 2 {
		t.Errorf("Dequeue = (%v, %v), expected N=2 unaffected", next, err)
	}
}

// TestPeekDoesNotRemove verifies that Peek leaves the queue unchanged and
// returns the same head each time.
func TestPeekDoesNotRemove(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	for i := range 3 {
		if v, err := q.Peek(); err != nil || v != 1 {
			t.Errorf("Peek step %d = (%v, %v), expected 1", i, v, err)
		}
	}
	if q.Length() != 2 {
		t.Errorf("Expected Peek to leave the length at 2, got %d", q.Length())
	}
	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Errorf("Dequeue after Peeks = (%v, %v), expected 1", v, err)
	}
}

// TestIteratorsAfterPartialDrain verifies that the iterators reflect the
// remaining window after some elements have been dequeued, with head at
// index 0.
func TestIteratorsAfterPartialDrain(t *testing.T) {
	var q Queue[int]
	for i := range 6 {
		q.Push(i)
	}
	for i := range 3 {
		if _, err := q.Dequeue(); err != nil {
			t.Fatalf("Dequeue(%d): %v", i, err)
		}
	}

	var fwd []int
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if expect := []int{3, 4, 5}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All after partial drain got %v, expected %v", fwd, expect)
	}

	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	if expect := []int{5, 4, 3}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward after partial drain got %v, expected %v", bwd, expect)
	}

	// Backward must be the exact reverse of All.
	for i := range fwd {
		if bwd[len(bwd)-1-i] != fwd[i] {
			t.Fatalf("Backward does not mirror All at %d", i)
		}
	}
}

// TestTruncateOnEmpty verifies truncating an empty queue is a no-op that
// leaves the queue usable.
func TestTruncateOnEmpty(t *testing.T) {
	var q Queue[int]
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate on empty queue.")
	}
	q.Push(7)
	if v, err := q.Dequeue(); err != nil || v != 7 {
		t.Errorf("Dequeue after truncate-on-empty = (%v, %v), expected 7", v, err)
	}
}

// TestQueuePropertyRandomized runs thousands of mixed operations against
// a slice reference model with a fixed seed, cross-checking after every
// step: FIFO order, length, and Peek agreement.
func TestQueuePropertyRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 11))
	const ops = 4000
	const keySpace = 50

	var q Queue[int]
	var model []int

	check := func(step int) {
		t.Helper()
		if q.Length() != len(model) {
			t.Fatalf("step %d: length %d, model has %d", step, q.Length(), len(model))
		}
		if got := contents(&q); !reflect.DeepEqual(got, model) {
			t.Fatalf("step %d: contents %v, model %v", step, got, model)
		}
		if len(model) > 0 {
			if v, err := q.Peek(); err != nil || v != model[0] {
				t.Fatalf("step %d: Peek = (%v, %v), model head %d", step, v, err, model[0])
			}
		}
	}

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(3) {
		case 0: // Push
			q.Push(v)
			model = append(model, v)
		case 1: // Dequeue
			got, err := q.Dequeue()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: Dequeue on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[0] {
					t.Fatalf("step %d: Dequeue = (%v, %v), model head %d", step, got, err, model[0])
				}
				model = model[1:]
			}
		case 2: // Pop (discard)
			err := q.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: Pop: %v", step, err)
				}
				model = model[1:]
			}
		}
		if step%50 == 0 {
			check(step)
		}
	}
	check(ops)

	// Final Backward cross-check.
	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	for i := range model {
		if bwd[len(model)-1-i] != model[i] {
			t.Fatalf("Final Backward mismatch at %d", i)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Thread-safety: snapshot iterators and concurrent access
// -------------------------------------------------------------------------------------------------------

// TestQueueIterateSnapshot verifies that the All/Backward iterators
// operate on a snapshot taken when they are called: later modifications —
// even truncating the whole queue — are not observed, and mutating the
// queue from inside the loop is safe.
func TestQueueIterateSnapshot(t *testing.T) {
	var q Queue[int]
	for i := range 5 {
		q.Push(i)
	}

	all := q.All()
	backward := q.Backward()

	q.Truncate() // the iterators above must not observe this

	var fwd []int
	for _, v := range all {
		fwd = append(fwd, v)
	}
	if expect := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All after Truncate error, expected %v got %v", expect, fwd)
	}

	var bwd []int
	for _, v := range backward {
		bwd = append(bwd, v)
	}
	for i := range fwd {
		if bwd[len(bwd)-1-i] != fwd[i] {
			t.Fatalf("Backward does not mirror All at %d", i)
		}
	}

	// Mutating from inside the loop is safe: the loop sees the snapshot.
	for i := range 3 {
		q.Push(i)
	}
	visited := 0
	for _, v := range q.All() {
		visited++
		_ = q.Pop()
		_ = v
	}
	if visited != 3 {
		t.Errorf("Expected 3 visits while popping during iteration, got %d", visited)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after popping during iteration.")
	}
}

// TestQueueConcurrent runs producers and consumers against one shared
// queue, with an accountant summing what flowed through.  It is primarily
// a test for the race detector (`make race`); the accounting must balance
// exactly: every produced element is dequeued exactly once.
func TestQueueConcurrent(t *testing.T) {
	var q Queue[int]

	const producers = 8
	const perProducer = 250
	const total = producers * perProducer

	var wg sync.WaitGroup
	var consumedCount atomic.Int64
	var consumedSum atomic.Int64

	// Consumers drain until the expected count has flowed through.  The
	// exit check must use the COUNT (exactly total items are ever
	// enqueued), not the value sum — comparing the sum against total made
	// the consumers quit after the first ~16 large values.
	for range 4 {
		wg.Go(func() {
			for {
				v, err := q.Dequeue()
				if err != nil {
					// Queue (temporarily) empty; spin until the count is
					// reached.
					if consumedCount.Load() >= total {
						return
					}
					continue
				}
				consumedCount.Add(1)
				consumedSum.Add(int64(v))
			}
		})
	}

	// Producers enqueue disjoint value ranges; the sum of all values is
	// deterministic even though the interleaving is not: the queue sees
	// each of 1..total exactly once.
	produced := int64(total) * int64(total+1) / 2
	for p := range producers {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := range perProducer {
				q.Enqueue(p*perProducer + i + 1) // values 1..total
			}
		}(p)
	}

	wg.Wait()

	if got := consumedCount.Load(); got != total {
		t.Errorf("Accounting mismatch: consumed %d items, expected %d", got, total)
	}
	if got := consumedSum.Load(); got != produced {
		t.Errorf("Accounting mismatch: consumed sum %d, expected %d", got, produced)
	}
	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty queue after concurrent drain, got length %d", q.Length())
	}
}
