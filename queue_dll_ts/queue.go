// Package queue_dll_ts implements a generic FIFO (first-in, first-out) queue
// on top of a generic doubly linked list (github.com/pschlump/pluto/dll_ts).
// The underlying generic type is thread safe, so this queue is safe for
// concurrent use.
//
// Unlike the slice based queues (../queue and ../queue_ts) Pop and Dequeue
// are O(1) with no memory copy cost.  Elements are stored as pointers: Push
// and Enqueue take a *T and Peek and Dequeue return a *T.
//
// Operations:
//
//	Push/Enqueue() — Inserts an element at the tail of the queue.			O(1)
//	Pop() — Removes the element at the head of the queue.					O(1)
//	Dequeue() — Removes and returns the element at the head of the queue.	O(1)
//	Peek() — Returns the element at the head of the queue without removing it. O(1)
//	IsEmpty() — Returns true if the queue is empty.							O(1)
//	Length() — Returns the number of elements in the queue.					O(1)
//	Truncate() — Removes all elements from the queue.						O(1)
//	All()/Backward() — Range-over-func iterators over the queue.			O(n)
//
// Note: This is a subset of the operations that are implemented on the
// `dll_ts` package.  This means that you can directly use ../dll_ts - but
// this may make code clearer that you are using a Queue instead.
//
// Copyright (C) Philip Schlump, 2012-2023.
// BSD 3 Clause Licensed.
package queue_dll_ts

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dll_ts"
)

// ErrEmptyQueue is the error returned by Pop, Peek and Dequeue when the queue is empty.
var ErrEmptyQueue = dll_ts.ErrEmptyDll

// Queue is a generic FIFO queue built on top of a generic doubly linked list.
//
// The zero value of Queue is an empty queue ready to use.
type Queue[T comparable.Equality] struct {
	data dll_ts.Dll[T]
}

// IsEmpty will return true if the queue is empty.
func (q *Queue[T]) IsEmpty() bool {
	return q.data.Length() == 0
}

// Push will push new data of type [T comparable.Equality] onto the tail of the queue.
func (q *Queue[T]) Push(t *T) {
	q.data.AppendAtTail(t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T comparable.Equality] onto the tail of the queue.
func (q *Queue[T]) Enqueue(t *T) {
	q.data.AppendAtTail(t)
}

// Pop will remove the head element from the queue.  An error is returned if the queue is empty.
func (q *Queue[T]) Pop() (err error) {
	_, err = q.data.Pop()
	return
}

// Dequeue removes and returns the head element from the queue (if there is one),
// else it returns an error.
func (q *Queue[T]) Dequeue() (rv *T, err error) {
	return q.data.Pop()
}

// Length returns the number of elements in the queue.
func (q *Queue[T]) Length() int {
	return q.data.Length()
}

// Peek returns the head element of the queue or an error indicating that the queue is empty.
func (q *Queue[T]) Peek() (*T, error) {
	return q.data.Peek()
}

// Truncate removes all of the data from the queue.
// Complexity is O(1).
func (q *Queue[T]) Truncate() {
	q.data.Truncate()
}

/* vim: set noai ts=4 sw=4: */
