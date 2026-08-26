/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package dqueue implements a generic double ended queue (deque) on top
// of a doubly linked list.
//
// Pluto has no plain dqueue — only the thread-safe dqueue_ts — so this
// package is charon's own: the design of dqueue_ts with the lock left
// out.  Like dqueue_ts (and stack_sll_ts) it is self-contained rather
// than a wrapper over dll: charon's dll requires an equality function
// at construction (its Search needs one), but a deque never compares
// elements — so wrapping would force a dummy equality function and
// break the constraint-free contract.  The prev pointers are what make
// PopBack and Backward O(1) per step.
//
// This implementation is NOT thread safe.  A mutex-guarded version with
// the exact same interface lives alongside it (see dqueue_ts).
//
// Elements can be inserted and removed at both ends of the queue; every
// such operation is strictly O(1):
//
//	PushFront — Inserts an element at the front.							O(1)
//	PushBack — Inserts an element at the back.							O(1)
//	PopFront — Removes and returns the front element.  ErrEmptyDeque	O(1)
//	          is returned if the queue is empty.
//	PopBack — Removes and returns the back element.  ErrEmptyDeque		O(1)
//	          is returned if the queue is empty.
//	PeekFront — Returns the front element without removing it.			O(1)
//	PeekBack — Returns the back element without removing it.				O(1)
//	IsEmpty — Returns true if the queue is empty.						O(1)
//	Len / Length — Returns the number of elements in the queue.			O(1)
//	Truncate — Removes all elements.										O(1)
//	All / Backward — Range-over-func iterators over the live list.		O(n)
//
// Because the deque is built on a linked list (not a slice window like
// the queue package), pushes and pops never reallocate: memory use is
// stable across arbitrary push/pop patterns, at the cost of one small
// node allocation per push.  Used at one end only it behaves as a stack;
// used with PushBack/PopFront it behaves as a FIFO queue (see ../queue
// and ../queue_dll).
//
// The element type needs no constraints at all: there is no ordering and
// no equality to supply, and the zero value of Deque is an empty deque
// ready to use — no constructor required.
//
// Errors, not panics, report the empty deque: ErrEmptyDeque.  Compare it
// with errors.Is.
//
// A nil *Deque behaves as an empty deque for every operation except
// PushFront and PushBack — a nil deque cannot store an element, and
// those calls panic with a message naming the method.  These are the
// package's only panics.
package dqueue

import (
	"errors"
)

// dequeElement is a node in the doubly linked list.
type dequeElement[T any] struct {
	data       T
	prev, next *dequeElement[T]
}

// Deque is a generic double ended queue built on top of a doubly linked
// list.
//
// The zero value of Deque is an empty deque, ready to use.
type Deque[T any] struct {
	head   *dequeElement[T]
	tail   *dequeElement[T]
	length int
}

// ErrEmptyDeque is returned by PopFront, PopBack, PeekFront and PeekBack
// when the deque is empty.
var ErrEmptyDeque = errors.New("empty deque")

// IsEmpty will return true if the deque is empty.
// Complexity is O(1).
func (q *Deque[T]) IsEmpty() bool {
	if q == nil {
		return true
	}
	return q.length == 0
}

// PushFront will push new data of type [T any] onto the front of the
// deque.  It panics on a nil deque — one of the package's two panics.
// Complexity is O(1).
func (q *Deque[T]) PushFront(t T) {
	if q == nil {
		panic("dqueue: PushFront called on a nil deque")
	}
	q.head = &dequeElement[T]{data: t, next: q.head}
	if q.tail == nil {
		q.tail = q.head
	} else {
		q.head.next.prev = q.head
	}
	q.length++
}

// PushBack will push new data of type [T any] onto the back of the
// deque.  It panics on a nil deque — one of the package's two panics.
// Complexity is O(1).
func (q *Deque[T]) PushBack(t T) {
	if q == nil {
		panic("dqueue: PushBack called on a nil deque")
	}
	e := &dequeElement[T]{data: t, prev: q.tail}
	if q.head == nil {
		q.head = e
	} else {
		q.tail.next = e
	}
	q.tail = e
	q.length++
}

// PopFront removes and returns the front element from the deque.
// ErrEmptyDeque is returned if the deque is empty.
//
// The element is returned by value; the deque no longer holds any
// reference to it.
// Complexity is O(1).
func (q *Deque[T]) PopFront() (rv T, err error) {
	if q == nil || q.length == 0 {
		return rv, ErrEmptyDeque
	}
	e := q.head
	q.head = e.next
	if q.head == nil {
		q.tail = nil
	} else {
		q.head.prev = nil
	}
	q.length--
	return e.data, nil
}

// PopBack removes and returns the back element from the deque.
// ErrEmptyDeque is returned if the deque is empty.
//
// The element is returned by value; the deque no longer holds any
// reference to it.
// Complexity is O(1).
func (q *Deque[T]) PopBack() (rv T, err error) {
	if q == nil || q.length == 0 {
		return rv, ErrEmptyDeque
	}
	e := q.tail
	q.tail = e.prev
	if q.tail == nil {
		q.head = nil
	} else {
		q.tail.next = nil
	}
	q.length--
	return e.data, nil
}

// PeekFront returns the front element of the deque or ErrEmptyDeque
// indicating that the deque is empty.
//
// The element is returned by value; it does not alias the deque's
// internals and cannot be invalidated by a later push or pop.
// Complexity is O(1).
func (q *Deque[T]) PeekFront() (rv T, err error) {
	if q == nil || q.length == 0 {
		return rv, ErrEmptyDeque
	}
	return q.head.data, nil
}

// PeekBack returns the back element of the deque or ErrEmptyDeque
// indicating that the deque is empty.
//
// The element is returned by value; it does not alias the deque's
// internals and cannot be invalidated by a later push or pop.
// Complexity is O(1).
func (q *Deque[T]) PeekBack() (rv T, err error) {
	if q == nil || q.length == 0 {
		return rv, ErrEmptyDeque
	}
	return q.tail.data, nil
}

// Len returns the number of elements in the deque.
// Complexity is O(1).
func (q *Deque[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// Length returns the number of elements in the deque.
// Complexity is O(1).
func (q *Deque[T]) Length() int {
	if q == nil {
		return 0
	}
	return q.length
}

// Truncate removes all of the data from the deque.
// Complexity is O(1).
func (q *Deque[T]) Truncate() {
	if q == nil {
		return
	}
	q.head = nil
	q.tail = nil
	q.length = 0
}
