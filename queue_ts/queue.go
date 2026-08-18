// Package queue implements a generic FIFO (first-in, first-out) queue
// on top of a slice.
//
// Queue is built with a slice so if the queue grows it will result in
// doubling of size and re-copy of data.
//
// This is the thread safe implementation.  All operations are guarded by a
// sync.RWMutex.  See github.com/pschlump/pluto/queue for a non-thread-safe
// version with the exact same interface, or
// github.com/pschlump/pluto/queue_dll_ts for an O(1) Pop/Dequeue
// implementation built on a doubly linked list.
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
//	All()/Backward() — Range-over-func iterators over a snapshot of the queue. O(n)
//
// Important See: https://medium.com/@cep21/gos-append-is-not-always-thread-safe-a3034db7975 for race stuff.
// Run the tests with the -race flag!
//
// Copyright (C) Philip Schlump, 2012-2023.
// BSD 3 Clause Licensed.
package queue

import (
	"errors"
	"sync"
)

// Queue is a generic FIFO queue built on top of a slice.
//
// The zero value of Queue is an empty queue ready to use.
type Queue[T any] struct {
	data []T
	lock sync.RWMutex
}

// ErrEmptyQueue is an error to indicate that the queue is empty.
var ErrEmptyQueue = errors.New("empty queue")

// IsEmpty will return true if the queue is empty.
func (q *Queue[T]) IsEmpty() bool {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.noLockIsEmpty()
}

// noLockIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (q *Queue[T]) noLockIsEmpty() bool {
	return len(q.data) == 0
}

// Push will push new data of type [T any] onto the tail of the queue.
func (q *Queue[T]) Push(t T) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = append(q.data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the tail of the queue.
func (q *Queue[T]) Enqueue(t T) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = append(q.data, t)
}

// Pop will remove the head element from the queue.  An error is returned if the queue is empty.
func (q *Queue[T]) Pop() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.noLockIsEmpty() {
		return ErrEmptyQueue
	}
	q.noLockPopHead()
	return nil
}

// Dequeue removes and returns the head element from the queue (if there is one),
// else it returns an error.
//
// The returned pointer refers to a copy of the element; the queue no longer
// holds any reference to it.
func (q *Queue[T]) Dequeue() (rv *T, err error) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.noLockIsEmpty() {
		return nil, ErrEmptyQueue
	}
	v := q.data[0]
	q.noLockPopHead()
	return &v, nil
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

// Length returns the number of elements in the queue.
func (q *Queue[T]) Length() int {
	q.lock.RLock()
	defer q.lock.RUnlock()
	return len(q.data)
}

// Peek returns the head element of the queue or an error indicating that the queue is empty.
//
// The returned pointer refers to the element inside the queue.  It is a
// snapshot in time: the element may be removed by another goroutine as soon
// as Peek returns.
func (q *Queue[T]) Peek() (*T, error) {
	q.lock.RLock()
	defer q.lock.RUnlock()
	if q.noLockIsEmpty() {
		return nil, ErrEmptyQueue
	}
	return &(q.data[0]), nil
}

// Truncate removes all of the data from the queue, releasing the backing array.
// Complexity is O(1).
func (q *Queue[T]) Truncate() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = nil
}

/* vim: set noai ts=4 sw=4: */
