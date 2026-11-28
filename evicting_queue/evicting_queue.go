/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package evicting_queue implements a fixed-capacity FIFO ring buffer. Push
// always succeeds and discards the oldest element when the queue is full.
package evicting_queue

import (
	"fmt"
	"iter"
)

// Queue is a fixed-capacity evicting FIFO queue. Its zero value has capacity
// zero and is read-only; use NewQueue before storing values.
type Queue[T any] struct {
	data   []T
	head   int
	length int
}

// NewQueue creates an empty queue. Capacity zero accepts and discards writes.
func NewQueue[T any](capacity int) *Queue[T] {
	if capacity < 0 {
		panic("evicting_queue: NewQueue called with a negative capacity")
	}
	return &Queue[T]{data: make([]T, capacity)}
}

func (q *Queue[T]) writeContract(caller string) {
	if q == nil || q.data == nil {
		panic(fmt.Sprintf("evicting_queue: %s on a nil or zero-value Queue; use NewQueue", caller))
	}
}

// Capacity returns the fixed maximum number of retained elements.
func (q *Queue[T]) Capacity() int {
	if q == nil {
		return 0
	}
	return len(q.data)
}

// RemainingCapacity returns how many pushes can occur before eviction starts.
func (q *Queue[T]) RemainingCapacity() int {
	return q.Capacity() - q.Len()
}

// Len returns the number of retained elements.
func (q *Queue[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// IsEmpty reports whether the queue contains no elements.
func (q *Queue[T]) IsEmpty() bool { return q.Len() == 0 }

// Push appends value, evicting the oldest value when full.
func (q *Queue[T]) Push(value T) {
	q.writeContract("Push")
	capacity := len(q.data)
	if capacity == 0 {
		return
	}
	if q.length == capacity {
		// Full: the new value overwrites the oldest slot, which becomes the tail.
		// The overwrite itself releases the evicted value.
		q.data[q.head] = value
		q.head = q.wrap(q.head + 1)
		return
	}
	q.data[q.wrap(q.head+q.length)] = value
	q.length++
}

// wrap reduces an index in [0, 2*capacity) into the ring. A compare-and-subtract
// replaces the integer division that % compiles to, which dominated iteration
// and the hot push/pop paths.
func (q *Queue[T]) wrap(index int) int {
	if index >= len(q.data) {
		index -= len(q.data)
	}
	return index
}

// segments returns the logical contents as at most two contiguous runs of the
// backing array, oldest first: data[head:head+n] and the wrapped prefix.
func (q *Queue[T]) segments() (first, second []T) {
	if q == nil || q.length == 0 {
		return nil, nil
	}
	end := q.head + q.length
	if end <= len(q.data) {
		return q.data[q.head:end], nil
	}
	return q.data[q.head:], q.data[:end-len(q.data)]
}

// PushAll appends values in order. Inputs at least as large as the capacity use
// a fast path that retains only the newest Capacity() inputs.
func (q *Queue[T]) PushAll(values []T) {
	if len(values) == 0 {
		return
	}
	q.writeContract("PushAll")
	capacity := len(q.data)
	if capacity == 0 {
		return
	}
	if len(values) >= capacity {
		var zero T
		for index := range q.data {
			q.data[index] = zero
		}
		copy(q.data, values[len(values)-capacity:])
		q.head = 0
		q.length = capacity
		return
	}
	for _, value := range values {
		if q.length == capacity {
			q.data[q.head] = value
			q.head = q.wrap(q.head + 1)
			continue
		}
		q.data[q.wrap(q.head+q.length)] = value
		q.length++
	}
}

// Pop removes and returns the oldest element.
func (q *Queue[T]) Pop() (T, bool) {
	if q == nil || q.length == 0 {
		var zero T
		return zero, false
	}
	value := q.data[q.head]
	var zero T
	q.data[q.head] = zero
	q.head = q.wrap(q.head + 1)
	q.length--
	if q.length == 0 {
		q.head = 0
	}
	return value, true
}

// Peek returns the oldest element without removing it.
func (q *Queue[T]) Peek() (T, bool) {
	if q == nil || q.length == 0 {
		var zero T
		return zero, false
	}
	return q.data[q.head], true
}

// Clear removes all elements and releases their references.
func (q *Queue[T]) Clear() {
	if q == nil {
		return
	}
	var zero T
	for index := range q.data {
		q.data[index] = zero
	}
	q.head = 0
	q.length = 0
}

// Slice returns a fresh oldest-to-newest snapshot.
func (q *Queue[T]) Slice() []T {
	if q == nil {
		return nil
	}
	first, second := q.segments()
	values := make([]T, q.length)
	copy(values[copy(values, first):], second)
	return values
}

// All iterates from oldest to newest. Do not mutate the queue while iterating.
func (q *Queue[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		first, second := q.segments()
		for _, value := range first {
			if !yield(value) {
				return
			}
		}
		for _, value := range second {
			if !yield(value) {
				return
			}
		}
	}
}

// Clone returns an independent queue with the same capacity and contents.
func (q *Queue[T]) Clone() *Queue[T] {
	if q == nil {
		return nil
	}
	clone := &Queue[T]{data: make([]T, len(q.data)), length: q.length}
	first, second := q.segments()
	copy(clone.data[copy(clone.data, first):], second)
	return clone
}

// Equal reports order-sensitive content equality. Elements must be comparable;
// unlike lengths and empty queues do not compare elements. Capacity is ignored.
func (q *Queue[T]) Equal(other *Queue[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	if q.Len() == 0 {
		return true
	}
	for index, position, otherPosition := 0, q.head, other.head; index < q.length; index++ {
		if any(q.data[position]) != any(other.data[otherPosition]) {
			return false
		}
		position, otherPosition = q.wrap(position+1), other.wrap(otherPosition+1)
	}
	return true
}
