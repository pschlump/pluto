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
	position := (q.head + q.length) % capacity
	if q.length == capacity {
		position = q.head
		var zero T
		q.data[position] = zero
		q.head = (q.head + 1) % capacity
	} else {
		q.length++
	}
	q.data[position] = value
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
		q.Push(value)
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
	q.head = (q.head + 1) % len(q.data)
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
	values := make([]T, q.length)
	for index := range values {
		values[index] = q.data[(q.head+index)%len(q.data)]
	}
	return values
}

// All iterates from oldest to newest. Do not mutate the queue while iterating.
func (q *Queue[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if q == nil {
			return
		}
		for index := 0; index < q.length; index++ {
			if !yield(q.data[(q.head+index)%len(q.data)]) {
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
	copy(clone.data, q.Slice())
	return clone
}

// Equal reports order-sensitive content equality. Elements must be comparable;
// unlike lengths and empty queues do not compare elements. Capacity is ignored.
func (q *Queue[T]) Equal(other *Queue[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	left, right := q.Slice(), other.Slice()
	for index := range left {
		if any(left[index]) != any(right[index]) {
			return false
		}
	}
	return true
}
