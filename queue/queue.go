// Package queue implements a generic FIFO (first-in, first-out) queue
// on top of a slice.
//
// This implementation is NOT thread safe.  See github.com/pschlump/pluto/queue_ts
// for a thread safe version with the exact same interface, or
// github.com/pschlump/pluto/queue_dll_ts for an O(1) Pop/Dequeue implementation
// built on a doubly linked list.
//
// Operations:
//
//	Push/Enqueue() — Inserts an element at the tail of the queue.			O(1) amortized
//	Pop() — Removes the element at the head of the queue.					O(1)
//	Dequeue() — Removes and returns the element at the head of the queue.	O(1)
//	Peek() — Returns the element at the head of the queue without removing it. O(1)
//	IsEmpty() — Returns true if the queue is empty.							O(1)
//	Length() — Returns the number of elements in the queue.					O(1)
//	Truncate() — Removes all elements from the queue.						O(1)
//	All()/Backward() — Range-over-func iterators over the queue.			O(n)
//
// Copyright (C) Philip Schlump, 2012-2021.
// BSD 3 Clause Licensed.
package queue

import (
	"errors"
)

// Queue is a generic FIFO queue built on top of a slice.
//
// The zero value of Queue is an empty queue ready to use.
type Queue[T any] struct {
	data []T
}

// ErrEmptyQueue is an error to indicate that the queue is empty.
var ErrEmptyQueue = errors.New("empty queue")

// IsEmpty will return true if the queue is empty.
func (q *Queue[T]) IsEmpty() bool {
	return len(q.data) == 0
}

// Push will push new data of type [T any] onto the tail of the queue.
func (q *Queue[T]) Push(t T) {
	q.data = append(q.data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the tail of the queue.
func (q *Queue[T]) Enqueue(t T) {
	q.data = append(q.data, t)
}

// Pop will remove the head element from the queue.  An error is returned if the queue is empty.
func (q *Queue[T]) Pop() error {
	if q.IsEmpty() {
		return ErrEmptyQueue
	}
	q.popHead()
	return nil
}

// Dequeue removes and returns the head element from the queue (if there is one),
// else it returns an error.
//
// The returned pointer refers to a copy of the element; the queue no longer
// holds any reference to it.
func (q *Queue[T]) Dequeue() (rv *T, err error) {
	if q.IsEmpty() {
		return nil, ErrEmptyQueue
	}
	v := q.data[0]
	q.popHead()
	return &v, nil
}

// popHead removes the head element, zeroing the vacated slot so that the
// backing array does not keep the element alive, and releasing the backing
// array entirely when the queue becomes empty.
func (q *Queue[T]) popHead() {
	var zero T
	q.data[0] = zero
	q.data = q.data[1:]
	if len(q.data) == 0 {
		q.data = nil
	}
}

// Length returns the number of elements in the queue.
func (q *Queue[T]) Length() int {
	return len(q.data)
}

// Peek returns the head element of the queue or an error indicating that the queue is empty.
//
// The returned pointer refers to the element inside the queue; mutating it
// mutates the queued element.
func (q *Queue[T]) Peek() (*T, error) {
	if q.IsEmpty() {
		return nil, ErrEmptyQueue
	}
	return &(q.data[0]), nil
}

// Truncate removes all of the data from the queue, releasing the backing array.
// Complexity is O(1).
func (q *Queue[T]) Truncate() {
	q.data = nil
}
