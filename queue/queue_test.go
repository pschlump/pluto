package queue

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

func TestQueue001(t *testing.T) {
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

	q.Push("a")
	q.Enqueue("b")
	q.Push("c")

	if q.IsEmpty() {
		t.Errorf("Expected non-empty queue after pushes.")
	}
	if q.Length() != 3 {
		t.Errorf("Expected length 3, got %d", q.Length())
	}

	// FIFO order.
	for i, want := range []string{"a", "b", "c"} {
		if v, err := q.Peek(); err != nil || v != want {
			t.Errorf("Peek step %d = (%q, %v), expected %q", i, v, err, want)
		}
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue step %d: %v", i, err)
		}
		if v != want {
			t.Errorf("Dequeue step %d = %q, expected %q", i, v, want)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after draining.")
	}
	if err := q.Pop(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue after draining, got %v", err)
	}
}

func TestEnqueueDequeue(t *testing.T) {
	q := &Queue[int]{}
	for i := range 100 {
		q.Enqueue(i)
	}
	if q.Length() != 100 {
		t.Fatalf("Expected length 100, got %d", q.Length())
	}
	for i := range 100 {
		v, err := q.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue(%d): %v", i, err)
		}
		if v != i {
			t.Fatalf("Dequeue = %d, expected %d", v, i)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after draining.")
	}
}

// TestPopReleasesBackingArray verifies that draining the queue releases
// the backing array entirely (data becomes nil), so a drained queue holds
// no reference to the popped elements.
func TestPopReleasesBackingArray(t *testing.T) {
	q := &Queue[int]{}
	for i := range 10 {
		q.Push(i)
	}
	for i := range 10 {
		if err := q.Pop(); err != nil {
			t.Fatalf("Pop(%d): %v", i, err)
		}
	}
	if q.data != nil {
		t.Errorf("Expected nil backing array after draining, got len=%d cap=%d", len(q.data), cap(q.data))
	}
}

func TestTruncate(t *testing.T) {
	q := &Queue[int]{}
	for i := range 10 {
		q.Push(i)
	}
	q.Truncate()

	if !q.IsEmpty() || q.Length() != 0 {
		t.Errorf("Expected empty queue after Truncate.")
	}
	if _, err := q.Peek(); !errors.Is(err, ErrEmptyQueue) {
		t.Errorf("Expected ErrEmptyQueue from Peek after Truncate, got %v", err)
	}

	// The queue is reusable after a truncate.
	q.Push(1)
	q.Push(2)
	if got, want := q.Length(), 2; got != want {
		t.Errorf("Expected length %d after refill, got %d", want, got)
	}
	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Errorf("Dequeue after refill = (%v, %v), expected 1", v, err)
	}

	// Truncating an already-empty queue is fine.
	q.Truncate()
	q.Truncate()
	if !q.IsEmpty() {
		t.Errorf("Expected empty queue after double Truncate.")
	}
}

func TestIterators(t *testing.T) {
	q := &Queue[string]{}
	for _, s := range []string{"a", "b", "c"} {
		q.Push(s)
	}

	var fwd []string
	for i, v := range q.All() {
		if i != len(fwd) {
			t.Fatalf("All: unexpected index %d at step %d", i, len(fwd))
		}
		fwd = append(fwd, v)
	}
	if expect := []string{"a", "b", "c"}; !reflect.DeepEqual(fwd, expect) {
		t.Errorf("All got %v, expected %v", fwd, expect)
	}

	var bwd []string
	for i, v := range q.Backward() {
		if i != 3-1-len(bwd) {
			t.Fatalf("Backward: unexpected index %d at step %d", i, len(bwd))
		}
		bwd = append(bwd, v)
	}
	if expect := []string{"c", "b", "a"}; !reflect.DeepEqual(bwd, expect) {
		t.Errorf("Backward got %v, expected %v", bwd, expect)
	}

	// Early break stops iteration.
	n := 0
	for range q.All() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}

	// Early break stops backward iteration too.
	n = 0
	for range q.Backward() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("Expected Backward early break to yield exactly 1 item, got %d", n)
	}

	// Iterating an empty queue yields nothing.
	empty := &Queue[int]{}
	for range empty.All() {
		t.Errorf("Expected no items from All on empty queue")
	}
	for range empty.Backward() {
		t.Errorf("Expected no items from Backward on empty queue")
	}
}

// -------------------------------------------------------------------------------------------------------
// Zero value and nil queue
// -------------------------------------------------------------------------------------------------------

// TestZeroValue exercises every operation on a freshly declared zero-value
// queue — including Push, which needs no constructor here.
func TestZeroValue(t *testing.T) {
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

	// The zero value is fully usable without a constructor: there is no
	// comparison or equality function to supply.
	q.Push(1)
	q.Push(2)
	if q.Length() != 2 {
		t.Errorf("Expected length 2 after pushes, got %d", q.Length())
	}
	if v, err := q.Dequeue(); err != nil || v != 1 {
		t.Errorf("Dequeue = (%v, %v), expected 1", v, err)
	}
}

// TestNilQueueTolerated verifies that every operation except Push/Enqueue
// treats a nil queue as an empty queue, and that Push/Enqueue panic with
// a message naming the method — the package's only panic.
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

	for name, fx := range map[string]func(){
		"Push":    func() { q.Push(1) },
		"Enqueue": func() { q.Enqueue(1) },
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("Expected %s on nil queue to panic.", name)
					return
				}
				if msg, ok := r.(string); !ok || !strings.Contains(msg, name) {
					t.Errorf("%s: unexpected panic message: %v", name, r)
				}
			}()
			fx()
		}()
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

const benchmarkQueueSize = 4096

func BenchmarkPush(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push(i)
	}
}

func BenchmarkPop(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if q.IsEmpty() {
			for j := range benchmarkQueueSize {
				q.Push(j)
			}
		}
		_ = q.Pop()
	}
}

func BenchmarkEnqueueDequeue(b *testing.B) {
	var q Queue[int]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(i)
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
