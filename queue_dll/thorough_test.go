package queue_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, link-integrity checks,
// duplicate edge cases, value-copy semantics of Peek, live-walk
// semantics of the iterators, truncate reuse, and a fixed-seed
// randomized property test.

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// checkModel verifies the queue's observable state against a reference
// model (a plain slice where the head is element 0).
func checkModel(t *testing.T, q *Queue[int], model []int) {
	t.Helper()
	if got, want := q.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := q.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	if len(model) > 0 {
		head, err := q.Peek()
		if err != nil {
			t.Fatalf("Peek on non-empty queue returned error: %v", err)
		}
		if head != model[0] {
			t.Fatalf("Peek: expected %d, got %d", model[0], head)
		}
	}
	// All must iterate head to tail.  slices.Equal (not reflect.DeepEqual)
	// so that a nil result and an emptied model compare equal.
	var fwd []int
	for _, v := range q.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	// Backward must iterate tail to head.
	var bwd []int
	for _, v := range q.Backward() {
		bwd = append(bwd, v)
	}
	var wantBwd []int
	for _, m := range slices.Backward(model) {
		wantBwd = append(wantBwd, m)
	}
	if !slices.Equal(bwd, wantBwd) {
		t.Fatalf("Backward: expected %v, got %v", wantBwd, bwd)
	}
}

// checkLinks verifies the structural invariants of the doubly linked
// list: the count matches Length, prev/next pointers are bidirectional,
// and head.prev / tail.next are nil.  Call it after structural changes.
func checkLinks[T any](t *testing.T, q *Queue[T]) {
	t.Helper()

	// Walk forward from the head, verifying every prev pointer on the way.
	n := 0
	var last *queueElement[T]
	for p := q.head; p != nil; p = p.next {
		if p.prev != last {
			t.Fatalf("link %d: p.prev = %v, expected %v", n, p.prev, last)
		}
		last = p
		n++
	}
	if last != q.tail {
		t.Fatalf("forward walk ended at %v, tail is %v", last, q.tail)
	}
	if n != q.length {
		t.Fatalf("forward walk counted %d nodes, length is %d", n, q.length)
	}

	// Walk backward from the tail, verifying every next pointer.
	n = 0
	var first *queueElement[T]
	for p := q.tail; p != nil; p = p.prev {
		if p.next != first && first != nil {
			t.Fatalf("link %d: p.next = %v, expected %v", n, p.next, first)
		}
		first = p
		n++
	}
	if first != q.head {
		t.Fatalf("backward walk ended at %v, head is %v", first, q.head)
	}
	if n != q.length {
		t.Fatalf("backward walk counted %d nodes, length is %d", n, q.length)
	}
}

// TestSingleElement covers the single-element edge case for both push
// aliases: pushing one element and dequeuing it must leave an empty
// queue with nil head and tail.
func TestSingleElement(t *testing.T) {
	for _, tc := range []struct {
		name string
		push func(q *Queue[string], v string)
	}{
		{"Push", (*Queue[string]).Push},
		{"Enqueue", (*Queue[string]).Enqueue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var q Queue[string]
			tc.push(&q, "x")

			if q.Length() != 1 {
				t.Fatalf("Expected length 1 got %d", q.Length())
			}
			checkLinks(t, &q)

			// Peek sees the single element.
			if p, err := q.Peek(); err != nil || p != "x" {
				t.Errorf("Peek = (%q, %v)", p, err)
			}

			v, err := q.Dequeue()
			if err != nil || v != "x" {
				t.Errorf("Dequeue = (%q, %v)", v, err)
			}
			if !q.IsEmpty() {
				t.Errorf("Expected empty queue after dequeuing the single element")
			}
			if q.head != nil || q.tail != nil {
				t.Errorf("Expected nil head and tail after draining, got %v/%v", q.head, q.tail)
			}
			checkLinks(t, &q)
		})
	}
}

// TestDuplicateValues verifies the queue stores duplicate values without
// conflating them.
func TestDuplicateValues(t *testing.T) {
	var q Queue[int]
	for range 5 {
		q.Push(7)
	}
	if q.Length() != 5 {
		t.Errorf("Expected length 5 got %d", q.Length())
	}
	n := 0
	for _, v := range q.All() {
		if v != 7 {
			t.Errorf("Expected 7 got %d", v)
		}
		n++
	}
	if n != 5 {
		t.Errorf("All: expected 5 elements got %d", n)
	}
	for i := range 5 {
		if v, err := q.Dequeue(); err != nil || v != 7 {
			t.Errorf("Dequeue step %d = (%d, %v)", i, v, err)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all duplicates")
	}
}

// TestPeekReturnsValue verifies that Peek returns an independent value:
// mutating it cannot affect the queue.
func TestPeekReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var q Queue[item]
	q.Push(item{S: "a", N: 1})
	q.Push(item{S: "b", N: 2})

	v, err := q.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	v.N = 99 // mutate the returned value

	if got, err := q.Peek(); err != nil || got.N != 1 {
		t.Errorf("Peek after mutation = (%v, %v), expected N=1 unaffected", got, err)
	}
	if q.Length() != 2 {
		t.Errorf("Expected the peek to leave the length at 2, got %d", q.Length())
	}
}

// TestPushDequeueInterleaved cross-checks against the model between
// every operation.
func TestPushDequeueInterleaved(t *testing.T) {
	var q Queue[int]
	var model []int

	for _, v := range []int{1, 2, 3} {
		q.Push(v)
		model = append(model, v)
		checkModel(t, &q, model)
	}

	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Fatalf("Dequeue = (%v, %v), expected 1", v, err)
	}
	model = model[1:]
	checkModel(t, &q, model)
	checkLinks(t, &q)

	if err := q.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	model = model[1:]
	checkModel(t, &q, model)
	checkLinks(t, &q)
}

// TestTruncateReuse verifies that the queue is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	var q Queue[int]
	for i := range 10 {
		q.Push(i)
	}
	q.Truncate()

	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty queue after Truncate.")
	}
	checkModel(t, &q, nil)
	checkLinks(t, &q)

	// Reusable after the drain.
	q.Push(7)
	if v, err := q.Peek(); err != nil || v != 7 {
		t.Errorf("Peek after Truncate = (%v, %v), expected 7", v, err)
	}

	// Truncating an already-empty queue is fine.
	q.Truncate()
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

// TestIteratorReflectsLiveQueue is the opposite of a snapshot iterator:
// the All/Backward iterators walk the live list — All through the next
// pointers, Backward through the prev pointers — so modifications made
// between iterations are visible, and the queue must not be modified
// while an iterator is running.
func TestIteratorReflectsLiveQueue(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	var seen []int
	for _, v := range q.All() {
		seen = append(seen, v)
		break // stop after the head element
	}
	if expect := []int{1}; !reflect.DeepEqual(seen, expect) {
		t.Fatalf("First visit: expected %v got %v", expect, seen)
	}

	// Push, then iterate again: the new tail is visible.
	q.Push(3)
	seen = nil
	for _, v := range q.All() {
		seen = append(seen, v)
	}
	if expect := []int{1, 2, 3}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("All after push: expected %v got %v", expect, seen)
	}

	if _, err := q.Dequeue(); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	seen = nil
	for _, v := range q.Backward() {
		seen = append(seen, v)
	}
	if expect := []int{3, 2}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("Backward after Dequeue: expected %v got %v", expect, seen)
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against
// a slice reference model with a fixed seed, cross-checking every
// observable result (length, peek, dequeue order, iteration order)
// along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260826, 7))
	const ops = 4000
	const keySpace = 50

	var q Queue[int]
	var model []int

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(4) {
		case 0, 1, 2: // Push (weighted so the queue grows)
			q.Push(v)
			model = append(model, v)
		case 3: // Dequeue
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
		}
		if step%50 == 0 {
			checkModel(t, &q, model)
			checkLinks(t, &q)
		}
	}
	checkModel(t, &q, model)
	checkLinks(t, &q)
}
