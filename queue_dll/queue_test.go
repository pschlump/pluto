package queue_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestQueue(t *testing.T) {
	var q Queue[string]

	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after declaration.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected length 0 after declaration, got %d/%d", q.Len(), q.Length())
	}

	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Pop on empty queue, got %v", err)
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Dequeue on empty queue, got %v", err)
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Peek on empty queue, got %v", err)
	}

	q.Push("hi")

	if q.IsEmpty() {
		t.Errorf("Expected non-empty queue after push.")
	}

	x, err := q.Dequeue()
	if err != nil {
		t.Errorf("Unexpected empty-queue error after 1 dequeue: %v", err)
	}
	if x != "hi" {
		t.Errorf("Expected %q got %q", "hi", x)
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue after draining, got %v", err)
	}

	q.Push("hi")
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after Truncate.")
	}

	q.Push("hi2")
	q.Enqueue("hi3")

	if got, want := q.Length(), 2; got != want {
		t.Errorf("Expected length of %d got %d", want, got)
	}

	ss, err := q.Peek()
	if err != nil {
		t.Errorf("Unexpected error on non-empty queue")
	}
	if ss != "hi2" {
		t.Errorf("Expected %s got %s", "hi2", ss)
	}
}

// TestFIFOOrder verifies the first-in-first-out contract: elements come
// out in exactly the order they went in, whichever alias pushed them.
func TestFIFOOrder(t *testing.T) {
	var q Queue[int]

	for i, v := range []int{10, 20, 30, 40} {
		if i%2 == 0 {
			q.Push(v)
		} else {
			q.Enqueue(v) // the same operation
		}
	}

	for i, expect := range []int{10, 20, 30, 40} {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Unexpected error on Dequeue: %s", err)
		}
		if v != expect {
			t.Errorf("Step %d: expected %d got %d", i, expect, v)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after dequeuing all elements")
	}
}

// TestPopDiscards verifies that Pop removes the head element without
// returning it, leaving the queue in the same state Dequeue would.
func TestPopDiscards(t *testing.T) {
	var q Queue[int]
	q.Push(1)
	q.Push(2)

	if err := q.Pop(); err != nil {
		t.Fatalf("Pop on non-empty queue returned %v", err)
	}
	if got, want := q.Length(), 1; got != want {
		t.Fatalf("After Pop: expected length %d got %d", want, got)
	}
	if v, err := q.Peek(); err != nil || v != 2 {
		t.Errorf("Peek after Pop = (%v, %v), expected 2", v, err)
	}
	if err := q.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after popping both elements")
	}
}

// TestQueueIterators covers All and Backward: order, indexes, early
// exit, and the empty queue.
func TestQueueIterators(t *testing.T) {
	var q Queue[int]
	for i := range 5 {
		q.Push(i)
	}

	// All: head to tail.
	var got []int
	for i, v := range q.All() {
		if i != len(got) {
			t.Errorf("All: expected index %d, got %d", len(got), i)
		}
		got = append(got, v)
	}
	if want := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("All: expected %v, got %v", want, got)
	}

	// All with early break.
	n := 0
	for range q.All() {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("All: expected early break after 2 elements, got %d", n)
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

	// Backward: tail to head, indexes matching All.
	got = got[:0]
	expect := 4
	for i, v := range q.Backward() {
		if i != expect {
			t.Errorf("Backward: expected index %d got %d", expect, i)
		}
		if v != expect {
			t.Errorf("Backward: expected value %d got %d", expect, v)
		}
		expect--
		got = append(got, v)
	}
	if want := []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("Backward: expected %v, got %v", want, got)
	}

	// Iterating an empty queue yields nothing.
	var empty Queue[int]
	for range empty.All() {
		t.Errorf("All: expected no elements on empty queue")
	}
	for range empty.Backward() {
		t.Errorf("Backward: expected no elements on empty queue")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value and nil queue
// -------------------------------------------------------------------------------------------------------

// TestZeroValueSemantics verifies that the zero value is a fully usable
// empty queue — no constructor, no constraints on the element type.
func TestZeroValueSemantics(t *testing.T) {
	var q Queue[int]

	if !q.IsEmpty() {
		t.Errorf("Expected zero value queue to be empty.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected zero value queue to have length 0.")
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Pop on zero value queue, got %v", err)
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Dequeue on zero value queue, got %v", err)
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Peek on zero value queue, got %v", err)
	}

	q.Push(1)
	q.Enqueue(2)
	if q.Length() != 2 {
		t.Errorf("Expected length 2 after pushes, got %d", q.Length())
	}
	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Errorf("Dequeue = (%v, %v), expected 1", v, err)
	}
	checkModel(t, &q, []int{2})
}

// TestNilQueueTolerated verifies that every operation except the pushes
// treats a nil queue as an empty queue, and that Push and Enqueue panic
// with messages naming the method — the package's only panics.
func TestNilQueueTolerated(t *testing.T) {
	var q *Queue[int]

	if !q.IsEmpty() {
		t.Errorf("Expected nil queue to be empty.")
	}
	if q.Len() != 0 || q.Length() != 0 {
		t.Errorf("Expected nil queue to have length 0.")
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Pop on nil queue, got %v", err)
	}
	if _, err := q.Dequeue(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Dequeue on nil queue, got %v", err)
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Peek on nil queue, got %v", err)
	}
	q.Truncate() // no-op, must not panic
	for range q.All() {
		t.Errorf("Expected no values from All on nil queue.")
	}
	for range q.Backward() {
		t.Errorf("Expected no values from Backward on nil queue.")
	}

	for _, tc := range []struct {
		name string
		call func()
	}{
		{"Push", func() { q.Push(1) }},
		{"Enqueue", func() { q.Enqueue(1) }},
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on nil queue to panic.", tc.name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, tc.name) {
					t.Errorf("%s: unexpected panic message: %v", tc.name, r)
				}
			}()
			tc.call()
		}()
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------------------------------------

const benchmarkQueueSize = 4096

func BenchmarkPush(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
}

func BenchmarkDequeue(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.IsEmpty() {
			for j := range benchmarkQueueSize {
				q.Push(j)
			}
		}
		_, _ = q.Dequeue()
	}
}

func BenchmarkPushDequeue(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(i)
		if _, err := q.Dequeue(); err != nil {
			b.Fatalf("Dequeue: %v", err)
		}
	}
}

func BenchmarkPeek(b *testing.B) {
	var q Queue[int]
	for i := range benchmarkQueueSize {
		q.Push(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = q.Peek()
	}
}
