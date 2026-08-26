/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package queue_dll implements a generic FIFO (first-in, first-out) queue
// on top of a doubly linked list.  It is a rework of
// github.com/pschlump/pluto/queue_dll_ts with the charon conventions —
// and without the lock: pluto has no plain queue_dll, so this is the
// plain package the ts-only pluto design never had.
//
// Pluto built its version as a thin wrapper delegating to the dll_ts
// package.  Charon's dll requires an equality function at construction
// (its Search needs one), but a queue never compares elements — so
// wrapping would force a dummy equality function and break the
// constraint-free contract.  Like dqueue (and stack_sll_ts) this package
// is self-contained, with its own plain doubly linked list underneath;
// the prev pointers are what make Backward O(1) per step.
//
// This implementation is NOT thread safe.  There is no queue_dll_ts
// twin (yet); a concurrent FIFO on the same list structure is
// dqueue_ts's PushBack/PopFront.
//
// Unlike the slice based queues (../queue) Pop and Dequeue are strictly
// O(1) with no reallocation: memory use is stable across arbitrary
// push/pop patterns, at the cost of one small node allocation per push.
// For a queue that also inserts and removes at the head, see ../dqueue
// — this package is dqueue's FIFO face.
//
// Operations:
//
//	Push/Enqueue() — Inserts an element at the tail of the queue.			O(1)
//	Pop() — Removes the element at the head of the queue.					O(1)
//	Dequeue() — Removes and returns the element at the head of the queue.	O(1)
//	Peek() — Returns the element at the head of the queue without removing it. O(1)
//	IsEmpty() — Returns true if the queue is empty.							O(1)
//	Len / Length() — Returns the number of elements in the queue.			O(1)
//	Truncate() — Removes all elements from the queue.						O(1)
//	All()/Backward() — Range-over-func iterators over the live list.		O(n)
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
package queue_dll

import (
	"errors"
)

// queueElement is a node in the doubly linked list.
type queueElement[T any] struct {
	data       T
	prev, next *queueElement[T]
}

// Queue is a generic FIFO queue built on top of a doubly linked list.
//
// The zero value of Queue is an empty queue, ready to use.
type Queue[T any] struct {
	head   *queueElement[T]
	tail   *queueElement[T]
	length int
}

// ErrEmptyQueue is the error returned by Pop, Peek and Dequeue when the
// queue is empty.
var ErrEmptyQueue = errors.New("empty queue")

// IsEmpty will return true if the queue is empty.
// Complexity is O(1).
func (q *Queue[T]) IsEmpty() bool {
	if q == nil {
		return true
	}
	return q.length == 0
}

// Push will push new data of type [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1).
func (q *Queue[T]) Push(t T) {
	if q == nil {
		panic("queue_dll: Push called on a nil queue")
	}
	e := &queueElement[T]{data: t, prev: q.tail}
	if q.head == nil {
		q.head = e
	} else {
		q.tail.next = e
	}
	q.tail = e
	q.length++
}

// Enqueue is the same as Push. Enqueue will push new data of type
// [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1).
func (q *Queue[T]) Enqueue(t T) {
	if q == nil {
		panic("queue_dll: Enqueue called on a nil queue")
	}
	e := &queueElement[T]{data: t, prev: q.tail}
	if q.head == nil {
		q.head = e
	} else {
		q.tail.next = e
	}
	q.tail = e
	q.length++
}

// popHead removes the head element, releasing it entirely — the node is
// unlinked, so neither the queue nor its iterators keep the element
// alive (there is no backing array to zero; a linked node is freed with
// its links).
func (q *Queue[T]) popHead() {
	e := q.head
	q.head = e.next
	if q.head == nil {
		q.tail = nil
	} else {
		q.head.prev = nil
	}
	q.length--
}

// Pop will remove the head element from the queue. ErrEmptyQueue is
// returned if the queue is empty.
// Complexity is O(1).
func (q *Queue[T]) Pop() (err error) {
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
	rv = q.head.data
	q.popHead()
	return rv, nil
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (q *Queue[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// Length returns the number of elements in the queue.
// Complexity is O(1).
func (q *Queue[T]) Length() int {
	if q == nil {
		return 0
	}
	return q.length
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
	return q.head.data, nil
}

// Truncate removes all of the data from the queue.
// Complexity is O(1).
func (q *Queue[T]) Truncate() {
	if q == nil {
		return
	}
	q.head = nil
	q.tail = nil
	q.length = 0
}
