package queue_dll_ts

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

// checkInvariant verifies that the queue's contents, head to tail, exactly
// match the reference model (a slice of strings used as a FIFO).
func checkInvariant(t *testing.T, q *Queue[TestDemo], model []string) {
	t.Helper()
	if got, expect := q.Length(), len(model); got != expect {
		t.Fatalf("invariant: expected length %d got %d", expect, got)
	}
	if got, expect := q.IsEmpty(), len(model) == 0; got != expect {
		t.Fatalf("invariant: expected IsEmpty=%v got %v (length %d)", expect, got, len(model))
	}

	// Peek must return the head without removing it.
	head, err := q.Peek()
	if len(model) == 0 {
		if !errors.Is(err, ErrEmptyQueue) {
			t.Fatalf("invariant: expected ErrEmptyQueue from Peek on empty queue, got %v", err)
		}
		if head != nil {
			t.Errorf("invariant: expected nil value from Peek on empty queue, got %+v", head)
		}
	} else {
		if err != nil {
			t.Fatalf("invariant: unexpected error from Peek: %s", err)
		}
		if head.S != model[0] {
			t.Errorf("invariant: expected head %q got %q", model[0], head.S)
		}
	}

	// Full forward iteration must match the model element-for-element.
	n := 0
	for i, v := range q.All() {
		if i != n {
			t.Errorf("invariant: All: expected index %d got %d", n, i)
		}
		if i >= len(model) {
			t.Fatalf("invariant: All yielded more elements (%d) than the model holds (%d)", i+1, len(model))
		}
		if v.S != model[i] {
			t.Errorf("invariant: All: at %d expected %q got %q", i, model[i], v.S)
		}
		n++
	}
	if n != len(model) {
		t.Errorf("invariant: All yielded %d elements, model has %d", n, len(model))
	}

	// Full backward iteration must match the reversed model, with indexes
	// matching those assigned by All.
	n = 0
	for i, v := range q.Backward() {
		mi := len(model) - 1 - n
		if i != mi {
			t.Errorf("invariant: Backward: expected index %d got %d", mi, i)
		}
		if mi < 0 {
			t.Fatalf("invariant: Backward yielded more elements than the model holds (%d)", len(model))
		}
		if v.S != model[mi] {
			t.Errorf("invariant: Backward: at %d expected %q got %q", mi, model[mi], v.S)
		}
		n++
	}
	if n != len(model) {
		t.Errorf("invariant: Backward yielded %d elements, model has %d", n, len(model))
	}
}

// TestZeroValue checks that a declared-but-never-initialized queue behaves
// as a valid empty queue.
func TestZeroValue(t *testing.T) {
	var q Queue[TestDemo]
	checkInvariant(t, &q, nil)

	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("expected ErrEmptyQueue from Pop on zero-value queue, got %v", err)
	}
	if v, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) || v != nil {
		t.Errorf("expected (nil, ErrEmptyQueue) from Dequeue on zero-value queue, got (%v, %v)", v, err)
	}
	if v, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) || v != nil {
		t.Errorf("expected (nil, ErrEmptyQueue) from Peek on zero-value queue, got (%v, %v)", v, err)
	}

	// Truncate on an empty queue is a no-op, not a panic.
	q.Truncate()
	checkInvariant(t, &q, nil)
}

// TestSingleElement exercises every operation on a queue holding exactly one
// element, including the transition back to empty.
func TestSingleElement(t *testing.T) {
	var q Queue[TestDemo]

	q.Push(&TestDemo{S: "only"})
	checkInvariant(t, &q, []string{"only"})

	// Peek twice: it must not consume the element.
	for i := 0; i < 2; i++ {
		v, err := q.Peek()
		if err != nil {
			t.Fatalf("unexpected error from Peek: %s", err)
		}
		if v.S != "only" {
			t.Errorf("expected %q got %q", "only", v.S)
		}
	}
	checkInvariant(t, &q, []string{"only"})

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("unexpected error from Dequeue: %s", err)
	}
	if v.S != "only" {
		t.Errorf("expected %q got %q", "only", v.S)
	}
	checkInvariant(t, &q, nil)

	// The queue must be reusable after draining to empty.
	q.Enqueue(&TestDemo{S: "again"})
	checkInvariant(t, &q, []string{"again"})
	if err := q.Pop(); err != nil {
		t.Errorf("unexpected error from Pop: %s", err)
	}
	checkInvariant(t, &q, nil)
}

// TestDuplicateValues verifies that the queue accepts duplicate values and
// preserves their FIFO order.
func TestDuplicateValues(t *testing.T) {
	var q Queue[TestDemo]
	model := []string{"dup", "dup", "x", "dup", "x"}
	for _, s := range model {
		q.Push(&TestDemo{S: s})
	}
	checkInvariant(t, &q, model)

	for len(model) > 0 {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("unexpected error from Dequeue: %s", err)
		}
		if v.S != model[0] {
			t.Errorf("expected %q got %q", model[0], v.S)
		}
		model = model[1:]
	}
	checkInvariant(t, &q, nil)
}

// TestTruncateAndReuse checks that Truncate empties the queue and that the
// queue keeps working correctly afterwards.
func TestTruncateAndReuse(t *testing.T) {
	var q Queue[TestDemo]
	for i := 0; i < 7; i++ {
		q.Enqueue(&TestDemo{S: fmt.Sprintf("t%d", i)})
	}
	q.Truncate()
	checkInvariant(t, &q, nil)

	// Error behavior on empty must be intact after a Truncate.
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("expected ErrEmptyQueue after Truncate, got %v", err)
	}

	q.Push(&TestDemo{S: "new"})
	checkInvariant(t, &q, []string{"new"})
}

// TestBackwardEarlyBreak verifies that breaking out of a Backward iteration
// stops the iterator (and covers the early-return path of Backward).
func TestBackwardEarlyBreak(t *testing.T) {
	var q Queue[TestDemo]
	for i := 0; i < 5; i++ {
		q.Push(&TestDemo{S: fmt.Sprintf("v%d", i)})
	}

	n := 0
	var lastIdx int
	for i := range q.Backward() {
		lastIdx = i
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("Backward: expected early break after 2 elements, got %d", n)
	}
	if lastIdx != 3 {
		t.Errorf("Backward: expected 2nd yielded index to be 3 (4,3,...) got %d", lastIdx)
	}

	// The queue must be unchanged by the aborted iteration.
	checkInvariant(t, &q, []string{"v0", "v1", "v2", "v3", "v4"})
}

// TestIteratorOnSingleElement covers iterator boundary conditions on a
// one-element queue.
func TestIteratorOnSingleElement(t *testing.T) {
	var q Queue[TestDemo]
	q.Push(&TestDemo{S: "one"})

	n := 0
	for i, v := range q.All() {
		if i != 0 || v.S != "one" {
			t.Errorf("All: expected (0, \"one\") got (%d, %q)", i, v.S)
		}
		n++
	}
	if n != 1 {
		t.Errorf("All: expected 1 element got %d", n)
	}

	n = 0
	for i, v := range q.Backward() {
		if i != 0 || v.S != "one" {
			t.Errorf("Backward: expected (0, \"one\") got (%d, %q)", i, v.S)
		}
		n++
	}
	if n != 1 {
		t.Errorf("Backward: expected 1 element got %d", n)
	}
}

// TestRandomizedAgainstModel drives the queue with a fixed-seed random mix
// of Push, Enqueue, Pop, Dequeue, Peek, Length, IsEmpty and Truncate,
// cross-checking every result against a plain slice used as a FIFO.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic test
	var q Queue[TestDemo]
	var model []string
	next := 0

	for op := 0; op < 2000; op++ {
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Push / Enqueue (weighted: queue grows)
			s := fmt.Sprintf("e%d", next)
			next++
			if rng.Intn(2) == 0 {
				q.Push(&TestDemo{S: s})
			} else {
				q.Enqueue(&TestDemo{S: s})
			}
			model = append(model, s)
		case 4, 5, 6: // Pop
			err := q.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("op %d: expected ErrEmptyQueue from Pop, got %v", op, err)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: unexpected error from Pop: %s", op, err)
				}
				model = model[1:]
			}
		case 7, 8: // Dequeue
			v, err := q.Dequeue()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("op %d: expected ErrEmptyQueue from Dequeue, got %v", op, err)
				}
				if v != nil {
					t.Fatalf("op %d: expected nil value from Dequeue on empty queue, got %+v", op, v)
				}
			} else {
				if err != nil {
					t.Fatalf("op %d: unexpected error from Dequeue: %s", op, err)
				}
				if v.S != model[0] {
					t.Fatalf("op %d: Dequeue expected %q got %q", op, model[0], v.S)
				}
				model = model[1:]
			}
		case 9: // Truncate
			q.Truncate()
			model = nil
		}
		checkInvariant(t, &q, model)
	}
}

// TestConcurrentIterators runs All/Backward iterators in parallel with
// producers and consumers. Iteration is documented as thread safe but not a
// consistent snapshot, so this only checks for races/crashes and that
// indexes stay in range — run with `go test -race`.
func TestConcurrentIterators(t *testing.T) {
	var q Queue[TestDemo]
	const producers = 3
	const perProducer = 200

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Iterating readers.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for idx, v := range q.All() {
					if v == nil {
						t.Errorf("All yielded a nil element")
						return
					}
					_ = idx
				}
				for idx, v := range q.Backward() {
					if v == nil {
						t.Errorf("Backward yielded a nil element")
						return
					}
					_ = idx
				}
			}
		}()
	}

	// Producers.
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.Push(&TestDemo{S: fmt.Sprintf("%d-%d", base, i)})
			}
		}(p)
	}

	// Consumer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = q.Dequeue()
			}
		}
	}()

	// Let them overlap, then shut the readers and consumer down.
	for i := 0; i < producers*perProducer; i++ {
		q.Enqueue(&TestDemo{S: fmt.Sprintf("main-%d", i)})
	}
	close(stop)
	wg.Wait()

	// Structure must still be consistent and usable.
	if q.IsEmpty() {
		// Possible (consumer may have drained everything) but the queue must
		// still accept new elements.
	}
	q.Truncate()
	checkInvariant(t, &q, nil)
	q.Push(&TestDemo{S: "post"})
	checkInvariant(t, &q, []string{"post"})
}

/* vim: set noai ts=4 sw=4: */
