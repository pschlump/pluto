/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package queue implements a generic FIFO (first-in, first-out) queue
// on top of a slice.
//
// Elements are stored and returned by value — Dequeue and Peek return
// (T, error) instead of a pointer — so an element never aliases the
// queue's internals.
//
// This implementation is NOT thread safe.  A mutex-guarded version with
// the exact same interface lives alongside it.
//
// Operations:
//
//	Push/Enqueue() — Inserts an element at the tail of the queue.			O(1) amortized
//	Pop() — Removes the element at the head of the queue.					O(1)
//	Dequeue() — Removes and returns the element at the head of the queue.	O(1)
//	Peek() — Returns the element at the head of the queue without removing it. O(1)
//	IsEmpty() — Returns true if the queue is empty.							O(1)
//	Len / Length() — Returns the number of elements in the queue.			O(1)
//	Truncate() — Removes all elements from the queue.						O(1)
//	All()/Backward() — Range-over-func iterators over the queue.			O(n)
//
// The element type needs no constraints at all: there is no ordering and
// no equality to supply, and the zero value of Queue is an empty queue
// ready to use — no constructor required.
//
// Errors, not panics, report the empty queue: ErrEmptyQueue.  Compare it
// with errors.Is.
//
// A nil *Queue behaves as an empty queue for every operation except
// Push/Enqueue — a nil queue cannot store an element, and that call
// panics with a message naming the method.  This is the package's only
// panic.
//
// The queue is built on a slice and pops advance the slice window; a
// long-lived queue that grows and drains repeatedly will periodically
// reallocate as the window walks off the front of the backing array.  For
// a queue with stable O(1) memory behavior across arbitrary push/pop
// patterns, use the doubly-linked-list based queue in the dll package
// (Enqueue/Pop).
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
// Complexity is O(1).
func (q *Queue[T]) IsEmpty() bool {
	return q == nil || len(q.data) == 0
}

// Push will push new data of type [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1) amortized.
func (q *Queue[T]) Push(t T) {
	if q == nil {
		panic("queue: Push called on a nil queue")
	}
	q.data = append(q.data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1) amortized.
func (q *Queue[T]) Enqueue(t T) {
	if q == nil {
		panic("queue: Enqueue called on a nil queue")
	}
	q.data = append(q.data, t)
}

// Pop will remove the head element from the queue.  ErrEmptyQueue is
// returned if the queue is empty.
// Complexity is O(1).
func (q *Queue[T]) Pop() error {
	if q.IsEmpty() {
		return ErrEmptyQueue
	}
	q.popHead()
	return nil
}

// Dequeue removes and returns the head element from the queue (if there
// is one), else it returns ErrEmptyQueue.
//
// The element is returned by value; the queue no longer holds any
// reference to it.
// Complexity is O(1).
func (q *Queue[T]) Dequeue() (rv T, err error) {
	if q.IsEmpty() {
		return rv, ErrEmptyQueue
	}
	rv = q.data[0]
	q.popHead()
	return rv, nil
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

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (q *Queue[T]) Len() int {
	if q == nil {
		return 0
	}
	return len(q.data)
}

// Length returns the number of elements in the queue.
// Complexity is O(1).
func (q *Queue[T]) Length() int {
	if q == nil {
		return 0
	}
	return len(q.data)
}

// Peek returns the head element of the queue or ErrEmptyQueue indicating
// that the queue is empty.
//
// The element is returned by value; it does not alias the queue's
// internals.
// Complexity is O(1).
func (q *Queue[T]) Peek() (rv T, err error) {
	if q.IsEmpty() {
		return rv, ErrEmptyQueue
	}
	return q.data[0], nil
}

// Truncate removes all of the data from the queue, releasing the backing
// array.
// Complexity is O(1).
func (q *Queue[T]) Truncate() {
	if q == nil {
		return
	}
	q.data = nil
}
