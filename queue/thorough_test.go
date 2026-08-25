package queue

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"math/rand"
	"testing"
)

// TestZeroValue verifies that a zero-value Queue is immediately usable and
// that all operations behave correctly on it.
func TestZeroValue(t *testing.T) {
	var q Queue[int]

	if !q.IsEmpty() {
		t.Errorf("Expected zero-value queue to be empty")
	}
	if got := q.Length(); got != 0 {
		t.Errorf("Expected length of 0 on zero-value queue, got %d", got)
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue on Pop of zero-value queue, got %v", err)
	}
	if v, err := q.Dequeue(); v != nil || !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected (nil, ErrEmptyQueue) on Dequeue of zero-value queue, got (%v, %v)", v, err)
	}
	if v, err := q.Peek(); v != nil || !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected (nil, ErrEmptyQueue) on Peek of zero-value queue, got (%v, %v)", v, err)
	}

	// Zero value must be usable without a constructor.
	q.Push(7)
	if q.IsEmpty() || q.Length() != 1 {
		t.Errorf("Expected length of 1 after Push on zero-value queue, got %d", q.Length())
	}
	v, err := q.Dequeue()
	if err != nil || *v != 7 {
		t.Errorf("Expected 7 after Push then Dequeue, got %v, %v", v, err)
	}
}

// TestSingleElement exercises the boundary between empty and non-empty with
// exactly one element, using each removal operation.
func TestSingleElement(t *testing.T) {
	// Removal via Pop.
	var q1 Queue[string]
	q1.Push("only")
	if err := q1.Pop(); err != nil {
		t.Fatalf("Unexpected error on Pop: %s", err)
	}
	if !q1.IsEmpty() || q1.Length() != 0 {
		t.Errorf("Expected empty queue after popping the single element")
	}
	if err := q1.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue after queue drained, got %v", err)
	}

	// Removal via Dequeue returns the element and leaves the queue empty.
	var q2 Queue[string]
	q2.Enqueue("only")
	v, err := q2.Dequeue()
	if err != nil {
		t.Fatalf("Unexpected error on Dequeue: %s", err)
	}
	if *v != "only" {
		t.Errorf("Expected %q got %q", "only", *v)
	}
	if !q2.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing the single element")
	}

	// Peek must not remove the element.
	var q3 Queue[string]
	q3.Push("only")
	for i := 0; i < 3; i++ {
		p, err := q3.Peek()
		if err != nil || *p != "only" {
			t.Errorf("Peek %d: expected %q, got %v, %v", i, "only", p, err)
		}
		if q3.Length() != 1 {
			t.Errorf("Peek %d: expected length to stay 1, got %d", i, q3.Length())
		}
	}
}

// TestDuplicateValues verifies that duplicate values are kept as distinct
// elements in FIFO order.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 5; i++ {
		q.Push(42)
	}
	if got := q.Length(); got != 5 {
		t.Errorf("Expected length of 5 with duplicate values, got %d", got)
	}
	for i := 0; i < 5; i++ {
		v, err := q.Dequeue()
		if err != nil || *v != 42 {
			t.Errorf("Dequeue %d: expected 42, got %v, %v", i, v, err)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all duplicates")
	}
}

// TestDequeueReturnsCopy verifies the documented semantics: Dequeue returns a
// pointer to a copy, so mutating it does not affect the queue; Peek returns a
// live pointer into the queue, so mutating it mutates the queued element.
func TestDequeueReturnsCopy(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	v, err := q.Dequeue()
	if err != nil {
		t.Fatalf("Unexpected error on Dequeue: %s", err)
	}
	*v = 999 // mutate the returned copy

	head, err := q.Peek()
	if err != nil || *head != 2 {
		t.Errorf("Expected head of 2 after mutating dequeued copy, got %v, %v", head, err)
	}

	// Peek returns a live reference: mutating it mutates the queued element.
	*head = 200
	again, err := q.Peek()
	if err != nil || *again != 200 {
		t.Errorf("Expected mutation through Peek pointer to stick, got %v, %v", again, err)
	}
}

// TestIteratorsAfterPartialDrain verifies that both iterators reflect the
// current head/tail after some elements have been dequeued, and that
// Backward stops on early break.
func TestIteratorsAfterPartialDrain(t *testing.T) {
	var q Queue[int]
	for i := 0; i < 6; i++ {
		q.Push(i)
	}
	for i := 0; i < 3; i++ {
		if err := q.Pop(); err != nil {
			t.Fatalf("Unexpected error on Pop: %s", err)
		}
	}

	// All: head to tail, values 3,4,5 with indexes 0,1,2.
	want := 3
	n := 0
	for i, v := range q.All() {
		if v != want || i != n {
			t.Errorf("All: expected index/value %d/%d got %d/%d", n, want, i, v)
		}
		want++
		n++
	}
	if n != 3 {
		t.Errorf("All: expected 3 elements after partial drain, got %d", n)
	}

	// Backward: tail to head, values 5,4,3 with indexes 2,1,0.
	want = 5
	n = 0
	for i, v := range q.Backward() {
		if v != want || i != 2-n {
			t.Errorf("Backward: expected index/value %d/%d got %d/%d", 2-n, want, i, v)
		}
		want--
		n++
	}
	if n != 3 {
		t.Errorf("Backward: expected 3 elements after partial drain, got %d", n)
	}

	// Backward with early break.
	n = 0
	for range q.Backward() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("Backward: expected early break after 2 elements, got %d", n)
	}
}

// TestTruncateOnEmpty verifies Truncate is safe on an empty queue and that
// repeated truncation keeps the queue usable.
func TestTruncateOnEmpty(t *testing.T) {
	var q Queue[int]
	q.Truncate()
	q.Truncate()
	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty queue after Truncate on empty queue")
	}
	q.Push(1)
	q.Truncate()
	q.Truncate()
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue after Truncate, got %v", err)
	}
}

// TestQueuePropertyRandomized cross-checks the queue against a plain slice
// reference model over hundreds of mixed, randomly chosen operations with a
// fixed seed.
func TestQueuePropertyRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(20240501))
	var q Queue[int]
	var model []int

	check := func(step int) {
		t.Helper()
		if q.Length() != len(model) {
			t.Fatalf("step %d: length mismatch: queue=%d model=%d", step, q.Length(), len(model))
		}
		if q.IsEmpty() != (len(model) == 0) {
			t.Fatalf("step %d: IsEmpty mismatch: queue=%v model=%v", step, q.IsEmpty(), len(model) == 0)
		}
		// Structural invariant: iterators must yield exactly the model contents.
		i := 0
		for idx, v := range q.All() {
			if idx != i || v != model[i] {
				t.Fatalf("step %d: All mismatch at %d: got (%d,%d) want (%d,%d)", step, i, idx, v, i, model[i])
			}
			i++
		}
		if i != len(model) {
			t.Fatalf("step %d: All yielded %d elements, model has %d", step, i, len(model))
		}
		i = len(model) - 1
		for idx, v := range q.Backward() {
			if idx != i || v != model[i] {
				t.Fatalf("step %d: Backward mismatch at %d: got (%d,%d) want (%d,%d)", step, i, idx, v, i, model[i])
			}
			i--
		}
		if i != -1 {
			t.Fatalf("step %d: Backward stopped at model index %d, expected -1", step, i)
		}
	}

	for step := 0; step < 2000; step++ {
		switch rng.Intn(6) {
		case 0, 1: // Push
			v := rng.Intn(100)
			q.Push(v)
			model = append(model, v)
		case 2: // Enqueue
			v := rng.Intn(100)
			q.Enqueue(v)
			model = append(model, v)
		case 3: // Pop
			err := q.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: expected ErrEmptyQueue on Pop of empty queue, got %v", step, err)
				}
			} else {
				if err != nil {
					t.Fatalf("step %d: unexpected error on Pop: %s", step, err)
				}
				model = model[1:]
			}
		case 4: // Dequeue / Peek
			if len(model) == 0 {
				if v, err := q.Dequeue(); v != nil || !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: expected (nil, ErrEmptyQueue) on Dequeue of empty queue, got (%v, %v)", step, v, err)
				}
				if v, err := q.Peek(); v != nil || !errors.Is(err, ErrEmptyQueue) {
					t.Fatalf("step %d: expected (nil, ErrEmptyQueue) on Peek of empty queue, got (%v, %v)", step, v, err)
				}
			} else {
				p, err := q.Peek()
				if err != nil || *p != model[0] {
					t.Fatalf("step %d: Peek got (%v, %v) want (%d, nil)", step, p, err, model[0])
				}
				v, err := q.Dequeue()
				if err != nil || *v != model[0] {
					t.Fatalf("step %d: Dequeue got (%v, %v) want (%d, nil)", step, v, err, model[0])
				}
				model = model[1:]
			}
		case 5: // Truncate
			q.Truncate()
			model = nil
		}
		check(step)
	}
}
