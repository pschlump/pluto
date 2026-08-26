/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package queue_ts implements a generic FIFO (first-in, first-out) queue
// on top of a slice that is safe for concurrent use.  It is the
// thread-safe twin of github.com/pschlump/charon/queue — the same API,
// guarded by a sync.RWMutex.
//
// Like every charon package it is a rework of its pluto counterpart
// (github.com/pschlump/pluto/queue_ts) with the charon conventions:
// elements are stored and returned by value — Dequeue and Peek return
// (T, error) instead of a pointer — so a returned element is an
// independent copy that cannot race with a concurrent pop zeroing the
// slot.
//
// Concurrency model:
//
//	Reads (Peek, Len, Length, IsEmpty) take the read lock and release it
//	before returning, so they run in parallel with each other.
//	Writes (Push, Enqueue, Pop, Dequeue, Truncate) take the write lock.
//	All and Backward operate on a snapshot copied under the read lock
//	when they are called, so they are safe to use concurrently with any
//	queue operation — including mutating the queue from inside the loop —
//	and never observe later modifications.
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
// Run the tests with -race.
package queue_ts

import (
	"errors"
	"sync"
)

// Queue is a generic FIFO queue built on top of a slice, safe for
// concurrent use.
//
// The zero value of Queue is an empty queue ready to use.
type Queue[T any] struct {
	data []T
	lock sync.RWMutex
}

// ErrEmptyQueue is an error to indicate that the queue is empty.
var ErrEmptyQueue = errors.New("empty queue")

// IsEmpty will return true if the queue is empty.
// Complexity is O(1).
func (q *Queue[T]) IsEmpty() bool {
	if q == nil {
		return true
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.noLockIsEmpty()
}

// noLockIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (q *Queue[T]) noLockIsEmpty() bool {
	return len(q.data) == 0
}

// Push will push new data of type [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1) amortized.
func (q *Queue[T]) Push(t T) {
	if q == nil {
		panic("queue_ts: Push called on a nil queue")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = append(q.data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the tail of the queue.
// It panics on a nil queue — the package's only panic.
// Complexity is O(1) amortized.
func (q *Queue[T]) Enqueue(t T) {
	if q == nil {
		panic("queue_ts: Enqueue called on a nil queue")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = append(q.data, t)
}

// Pop will remove the head element from the queue.  ErrEmptyQueue is
// returned if the queue is empty.
// Complexity is O(1).
func (q *Queue[T]) Pop() error {
	if q == nil {
		return ErrEmptyQueue
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.noLockIsEmpty() {
		return ErrEmptyQueue
	}
	q.noLockPopHead()
	return nil
}

// Dequeue removes and returns the head element from the queue (if there
// is one), else it returns ErrEmptyQueue.
//
// The element is returned by value; the queue no longer holds any
// reference to it.
// Complexity is O(1).
func (q *Queue[T]) Dequeue() (rv T, err error) {
	if q == nil {
		return rv, ErrEmptyQueue
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.noLockIsEmpty() {
		return rv, ErrEmptyQueue
	}
	rv = q.data[0]
	q.noLockPopHead()
	return rv, nil
}

// noLockPopHead removes the head element, zeroing the vacated slot so that the
// backing array does not keep the element alive, and releasing the backing
// array entirely when the queue becomes empty.  The caller must hold the lock.
func (q *Queue[T]) noLockPopHead() {
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
	q.lock.RLock()
	defer q.lock.RUnlock()
	return len(q.data)
}

// Length returns the number of elements in the queue.
// Complexity is O(1).
func (q *Queue[T]) Length() int {
	if q == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return len(q.data)
}

// Peek returns the head element of the queue or ErrEmptyQueue indicating
// that the queue is empty.
//
// The element is returned by value: it is an independent copy taken under
// the read lock, not a live view.  The element may of course be dequeued
// by another goroutine as soon as Peek returns.
// Complexity is O(1).
func (q *Queue[T]) Peek() (rv T, err error) {
	if q == nil {
		return rv, ErrEmptyQueue
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	if q.noLockIsEmpty() {
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
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = nil
}
