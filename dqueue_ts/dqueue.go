/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package dqueue_ts implements a generic, thread-safe double ended queue
// (deque) on top of a doubly linked list.
//
// The package is self-contained rather than a wrapper over the dll_ts
// package: pluto's dll_ts requires an equality function at
// construction (its Search needs one), but a deque never compares
// elements — so wrapping would force a dummy equality function and break
// the constraint-free contract.  Like every pluto _ts package this one
// has its own sync.RWMutex and a plain doubly linked list underneath
// (the prev pointers are what make PopBack and Backward O(1) per step).
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
//	All / Backward — Range-over-func iterators over a snapshot.			O(n)
//
// Because the deque is built on a linked list (not a slice window like
// the queue package), pushes and pops never reallocate: memory use is
// stable across arbitrary push/pop patterns, at the cost of one small
// node allocation per push.  Used at one end only it behaves as a stack;
// used with PushBack/PopFront it behaves as a FIFO queue (see ../queue_ts).
//
// Concurrency model:
//
//	Reads (PeekFront, PeekBack, Len, Length, IsEmpty) take the read lock
//	and release it before returning, so they run in parallel with each
//	other.  Writes (PushFront, PushBack, PopFront, PopBack, Truncate)
//	take the write lock.  All and Backward operate on a snapshot taken
//	when they are called (one O(n) copy, under the read lock), so they
//	are safe to use concurrently with any deque operation — including
//	mutating the deque from inside the loop — and never observe later
//	modifications.
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
//
// Run the tests with -race.
package dqueue_ts

import (
	"errors"
	"sync"
)

// dequeElement is a node in the doubly linked list.
type dequeElement[T any] struct {
	data       T
	prev, next *dequeElement[T]
}

// Deque is a generic, thread-safe double ended queue built on top of a
// doubly linked list.
//
// The zero value of Deque is an empty deque, ready to use.
type Deque[T any] struct {
	head   *dequeElement[T]
	tail   *dequeElement[T]
	length int
	lock   sync.RWMutex
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
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.length == 0
}

// PushFront will push new data of type [T any] onto the front of the
// deque.  It panics on a nil deque — one of the package's two panics.
// Complexity is O(1).
func (q *Deque[T]) PushFront(t T) {
	if q == nil {
		panic("dqueue_ts: PushFront called on a nil deque")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
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
		panic("dqueue_ts: PushBack called on a nil deque")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.noLockPushBack(t)
}

// noLockPushBack pushes t onto the back of the deque.  The caller must
// hold the write lock.
func (q *Deque[T]) noLockPushBack(t T) {
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
	if q == nil {
		return rv, ErrEmptyDeque
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.length == 0 {
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
	if q == nil {
		return rv, ErrEmptyDeque
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.length == 0 {
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
	if q == nil {
		return rv, ErrEmptyDeque
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	if q.length == 0 {
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
	if q == nil {
		return rv, ErrEmptyDeque
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	if q.length == 0 {
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
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.length
}

// Length returns the number of elements in the deque.
// Complexity is O(1).
func (q *Deque[T]) Length() int {
	if q == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.length
}

// Truncate removes all of the data from the deque.
// Complexity is O(1).
func (q *Deque[T]) Truncate() {
	if q == nil {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.head = nil
	q.tail = nil
	q.length = 0
}

// snapshot returns the data of the deque from front to back, taken under
// the read lock.  A nil deque yields nil.  The caller must NOT hold the
// lock.
// Complexity is O(n).
func (q *Deque[T]) snapshot() []T {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	items := make([]T, 0, q.length)
	for p := q.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return items
}
