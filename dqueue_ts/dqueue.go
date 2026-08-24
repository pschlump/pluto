// Package dqueue_ts implements a generic double ended queue (deque) on top of
// a generic doubly linked list (github.com/pschlump/pluto/dll_ts).
// The underlying generic type is thread safe, so this deque is safe for
// concurrent use.
//
// Elements can be inserted and removed at both ends of the queue.  Every
// operation is O(1) with no memory copy cost.  Elements are stored as
// pointers: PushFront and PushBack take a *T and PeekFront, PeekBack,
// PopFront and PopBack return a *T.
//
// Operations:
//
//	PushFront() — Inserts an element at the front of the queue.				O(1)
//	PushBack() — Inserts an element at the back of the queue.				O(1)
//	PopFront() — Removes and returns the element at the front of the queue.	O(1)
//	PopBack() — Removes and returns the element at the back of the queue.	O(1)
//	PeekFront() — Returns the front element without removing it.			O(1)
//	PeekBack() — Returns the back element without removing it.				O(1)
//	IsEmpty() — Returns true if the queue is empty.							O(1)
//	Length() — Returns the number of elements in the queue.					O(1)
//	Truncate() — Removes all elements from the queue.						O(1)
//	All()/Backward() — Range-over-func iterators over the queue.			O(n)
//
// Note: This is a subset of the operations that are implemented on the
// `dll_ts` package.  This means that you can directly use ../dll_ts - but
// this may make code clearer that you are using a double ended queue
// instead.
//
// Copyright (C) Philip Schlump, 2012-2023.
// BSD 3 Clause Licensed.
package dqueue_ts

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dll_ts"
)

// ErrEmptyDeque is the error returned by PopFront, PopBack, PeekFront and
// PeekBack when the queue is empty.
var ErrEmptyDeque = dll_ts.ErrEmptyDll

// Deque is a generic double ended queue built on top of a generic doubly
// linked list.
//
// The zero value of Deque is an empty queue ready to use.
type Deque[T comparable.Equality] struct {
	data dll_ts.Dll[T]
}

// IsEmpty will return true if the queue is empty.
func (q *Deque[T]) IsEmpty() bool {
	return q.data.Length() == 0
}

// PushFront will push new data of type [T comparable.Equality] onto the
// front of the queue.
func (q *Deque[T]) PushFront(t *T) {
	q.data.InsertBeforeHead(t)
}

// PushBack will push new data of type [T comparable.Equality] onto the
// back of the queue.
func (q *Deque[T]) PushBack(t *T) {
	q.data.AppendAtTail(t)
}

// PopFront removes and returns the front element from the queue (if there
// is one), else it returns an error.
func (q *Deque[T]) PopFront() (rv *T, err error) {
	return q.data.Pop()
}

// PopBack removes and returns the back element from the queue (if there
// is one), else it returns an error.
func (q *Deque[T]) PopBack() (rv *T, err error) {
	return q.data.PopTail()
}

// PeekFront returns the front element of the queue or an error indicating
// that the queue is empty.
func (q *Deque[T]) PeekFront() (*T, error) {
	return q.data.Peek()
}

// PeekBack returns the back element of the queue or an error indicating
// that the queue is empty.
func (q *Deque[T]) PeekBack() (*T, error) {
	return q.data.PeekTail()
}

// Length returns the number of elements in the queue.
func (q *Deque[T]) Length() int {
	return q.data.Length()
}

// Truncate removes all of the data from the queue.
// Complexity is O(1).
func (q *Deque[T]) Truncate() {
	q.data.Truncate()
}

/* vim: set noai ts=4 sw=4: */
