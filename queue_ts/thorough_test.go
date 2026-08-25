package queue

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"math/rand"
	"testing"
)

// TestZeroValue verifies that a freshly declared (zero-value) queue behaves
// as a valid empty queue: reads report empty, removals error out, and it is
// immediately usable without a constructor.
func TestZeroValue(t *testing.T) {
	var q Queue[int]

	if !q.IsEmpty() {
		t.Errorf("zero value: expected IsEmpty to be true")
	}
	if got := q.Length(); got != 0 {
		t.Errorf("zero value: expected Length 0 got %d", got)
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("zero value: expected ErrEmptyQueue on Pop, got %v", err)
	}
	if v, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) || v != nil {
		t.Errorf("zero value: expected (nil, ErrEmptyQueue) on Dequeue, got (%v, %v)", v, err)
	}
	if v, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) || v != nil {
		t.Errorf("zero value: expected (nil, ErrEmptyQueue) on Peek, got (%v, %v)", v, err)
	}
	for range q.All() {
		t.Errorf("zero value: expected All to yield nothing")
	}
	for range q.Backward() {
		t.Errorf("zero value: expected Backward to yield nothing")
	}

	// Zero value must be usable without any initialization.
	q.Push(42)
	if q.IsEmpty() || q.Length() != 1 {
		t.Errorf("zero value: expected one element after Push")
	}
}

// TestSingleElement exercises the one-element edge case through every
// operation and verifies the queue returns to a clean empty state.
func TestSingleElement(t *testing.T) {
	var q Queue[string]

	q.Enqueue("only")
	if q.IsEmpty() {
		t.Errorf("expected non-empty queue after Enqueue")
	}
	if got := q.Length(); got != 1 {
		t.Errorf("expected Length 1 got %d", got)
	}

	p, err := q.Peek()
	if err != nil {
		t.Fatalf("unexpected error on Peek: %s", err)
	}
	if *p != "only" {
		t.Errorf("expected Peek %q got %q", "only", *p)
	}
	// Peek must not remove the element.
	if got := q.Length(); got != 1 {
		t.Errorf("expected Length still 1 after Peek, got %d", got)
	}

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("unexpected error on Dequeue: %s", err)
	}
	if *v != "only" {
		t.Errorf("expected Dequeue %q got %q", "only", *v)
	}
	if !q.IsEmpty() {
		t.Errorf("expected empty queue after Dequeue of last element")
	}
	if got := q.Length(); got != 0 {
		t.Errorf("expected Length 0 after draining, got %d", got)
	}

	// Queue must still be usable after being fully drained.
	q.Push("again")
	if err := q.Pop(); err != nil {
		t.Errorf("unexpected error on Pop: %s", err)
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("expected ErrEmptyQueue on second Pop, got %v", err)
	}
}

// TestDuplicateValues verifies that the queue accepts and preserves duplicate
// values in FIFO order.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]

	for i := 0; i < 5; i++ {
		q.Push(7)
	}
	if got := q.Length(); got != 5 {
		t.Fatalf("expected Length 5 got %d", got)
	}
	for i := 0; i < 5; i++ {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("unexpected error on Dequeue: %s", err)
		}
		if *v != 7 {
			t.Errorf("expected 7 got %d", *v)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("expected empty queue after removing all duplicates")
	}
}

// TestDequeueReturnsCopy verifies that the pointer returned by Dequeue refers
// to a copy: mutating it must not affect elements still in the queue.
func TestDequeueReturnsCopy(t *testing.T) {
	var q Queue[int]

	q.Push(1)
	q.Push(2)

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("unexpected error on Dequeue: %s", err)
	}
	*v = 999

	p, err := q.Peek()
	if err != nil {
		t.Fatalf("unexpected error on Peek: %s", err)
	}
	if *p != 2 {
		t.Errorf("expected head 2 after mutating dequeued copy, got %d", *p)
	}
}

// TestTruncateReuse verifies Truncate empties the queue and that the queue
// remains fully usable afterwards.
func TestTruncateReuse(t *testing.T) {
	var q Queue[int]

	for i := 0; i < 10; i++ {
		q.Push(i)
	}
	q.Truncate()

	if !q.IsEmpty() {
		t.Errorf("expected empty queue after Truncate")
	}
	if got := q.Length(); got != 0 {
		t.Errorf("expected Length 0 after Truncate, got %d", got)
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("expected ErrEmptyQueue after Truncate, got %v", err)
	}
	for range q.All() {
		t.Errorf("expected All to yield nothing after Truncate")
	}

	// Truncating an already-empty queue must be harmless.
	q.Truncate()

	for i := 0; i < 3; i++ {
		q.Push(i)
	}
	v, err := q.Dequeue()
	if err != nil || *v != 0 {
		t.Errorf("expected first Dequeue after Truncate to be 0, got %v, %v", v, err)
	}
}

// TestIteratorEarlyBreak verifies that breaking out of the range loop stops
// iteration (the yield-false path) for both All and Backward.
func TestIteratorEarlyBreak(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 10; i++ {
		q.Push(i)
	}

	n := 0
	for i, v := range q.All() {
		if i != v {
			t.Errorf("All: expected index %d to match value %d", i, v)
		}
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Errorf("All: expected iteration to stop after 3 yields, got %d", n)
	}

	n = 0
	for i, v := range q.Backward() {
		if i != v || v != 9-n {
			t.Errorf("Backward: expected index/value %d got %d/%d", 9-n, i, v)
		}
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Errorf("Backward: expected iteration to stop after 3 yields, got %d", n)
	}
}

// TestIteratorSnapshot verifies that All and Backward iterate a snapshot
// taken when the iterator is created: mutations made before the range loop
// starts do not appear, and mutations during iteration do not disturb it.
func TestIteratorSnapshot(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 4; i++ {
		q.Push(i)
	}

	// Snapshot is taken at the call to All, before the loop body runs.
	seq := q.All()
	q.Push(4)
	_ = q.Pop()

	n := 0
	for i, v := range seq {
		if i != v {
			t.Errorf("All snapshot: expected index %d to match value %d", i, v)
		}
		n++
	}
	if n != 4 {
		t.Errorf("All snapshot: expected the original 4 elements, got %d", n)
	}

	// Mutating during iteration must not disturb a Backward snapshot either.
	seqB := q.Backward()
	first := true
	for range seqB {
		if first {
			q.Push(100)
			first = false
		}
		n++
	}
	if n != 8 {
		t.Errorf("Backward snapshot: expected 4 more elements, total %d", n)
	}
}

// TestIteratorEmptyAfterTruncate verifies iterators created on an emptied
// queue yield nothing.
func TestIteratorEmptyAfterTruncate(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 5; i++ {
		q.Push(i)
	}
	q.Truncate()
	for range q.All() {
		t.Errorf("All: expected no elements after Truncate")
	}
	for range q.Backward() {
		t.Errorf("Backward: expected no elements after Truncate")
	}
}

// TestMixedOperationsModel is a randomized property test (fixed seed) that
// cross-checks the queue against a plain-slice reference model over hundreds
// of mixed Push/Enqueue/Pop/Dequeue/Peek/Truncate operations.
func TestMixedOperationsModel(t *testing.T) {
	var q Queue[int]
	var model []int
	rng := rand.New(rand.NewSource(42))

	check := func(step int) {
		t.Helper()
		if got, expect := q.Length(), len(model); got != expect {
			t.Fatalf("step %d: expected Length %d got %d", step, expect, got)
		}
		if got, expect := q.IsEmpty(), len(model) == 0; got != expect {
			t.Fatalf("step %d: expected IsEmpty %v got %v", step, expect, got)
		}
		p, err := q.Peek()
		if len(model) == 0 {
			if !errors.Is(err, ErrEmptyQueue) {
				t.Fatalf("step %d: expected ErrEmptyQueue on Peek, got %v", step, err)
			}
		} else {
			if err != nil {
				t.Fatalf("step %d: unexpected error on Peek: %s", step, err)
			}
			if *p != model[0] {
				t.Fatalf("step %d: expected Peek %d got %d", step, model[0], *p)
			}
		}
	}

	for step := 0; step < 800; step++ {
		switch rng.Intn(6) {
		case 0:
			v := rng.Intn(1000)
			q.Push(v)
			model = append(model, v)
		case 1:
			v := rng.Intn(1000)
			q.Enqueue(v)
			model = append(model, v)
		case 2:
			err := q.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: expected ErrEmptyQueue on Pop, got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: unexpected error on Pop: %s", step, err)
				}
				model = model[1:]
			}
		case 3:
			v, err := q.Dequeue()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: expected ErrEmptyQueue on Dequeue, got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: unexpected error on Dequeue: %s", step, err)
				}
				if *v != model[0] {
					t.Fatalf("step %d: expected Dequeue %d got %d", step, model[0], *v)
				}
				model = model[1:]
			}
		case 4:
			q.Truncate()
			model = nil
		case 5:
			// Full forward iteration must match the model exactly.
			i := 0
			for _, v := range q.All() {
				if i >= len(model) || v != model[i] {
					t.Fatalf("step %d: All mismatch at %d: got %d, model %v", step, i, v, model)
				}
				i++
			}
			if i != len(model) {
				t.Fatalf("step %d: All yielded %d elements, model has %d", step, i, len(model))
			}
		}
		check(step)
	}
}
