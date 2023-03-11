package queue

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.

Basic operations on a Queue

Queue is built with a slice so if the queue grows it will result in
doubling of size and re-copy of data.

This is the thread safe implementation.

*	Enqueue() — Inserts an element to the end of the queue (Same as "Push")
*	Dequeue() — Removes an element from the start of the queue (Same as "Peek" then "Pop")
*	IsEmpty() — Returns true if the queue is empty												O(1)
*	Top() — Returns the first element of the queue (Same as "Peek")
*	Push() - Insert into the tail of the Queue.													O(1) with possible memory copy cost, element is added to end of slice.
*	Enqueue() - Insert into the tail of the Queue.  Same as Push()								O(1) with possible memory copy cost, element is added to end of slice.
* 	Truncate - Delete all the nodes in list. 													O(1)

Important See: https://medium.com/@cep21/gos-append-is-not-always-thread-safe-a3034db7975 for race stuff.
	-race flag on testing!

*/

import (
	"errors"
	"sync"
)

// Queue is a generic type buildt on top of a slice
type Queue[T any] struct {
	data []T
	lock sync.RWMutex
}

// IsEmpty will return true if the queue is empty
func (ns *Queue[T]) IsEmpty() bool {
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return len((*ns).data) == 0
}

func (ns *Queue[T]) nlIsEmpty() bool {
	return len((*ns).data) == 0
}

// Push will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Push(t T) {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	(*ns).data = append((*ns).data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Enqueue(t T) {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	(*ns).data = append((*ns).data, t)
}

// An error to indicate that the queue is empty
var ErrEmptyQueue = errors.New("Empty Queue")

// Pop will remove the top element from the queue.  An error is returned if the queue is empty.
func (ns *Queue[T]) Pop() error {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if ns.nlIsEmpty() {
		return ErrEmptyQueue
	}
	// (*ns).data = (*ns).data[1:len((*ns).data)]
	(*ns).data = (*ns).data[1:]
	return nil
}

// Length returns the number of elements in the queue.
func (ns *Queue[T]) Length() int {
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return len((*ns).data)
}

// Peek returns the top element of the queue or an error indicating that the queue is empty.
func (ns *Queue[T]) Peek() (*T, error) {
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if !ns.nlIsEmpty() {
		return &((*ns).data[0]), nil
	}
	return nil, ErrEmptyQueue
}

// Dequeue remove and return an element from the queue (if there is one), else return an error.
func (ns *Queue[T]) Dequeue() (rv *T, err error) {
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.nlIsEmpty() {
		err = ErrEmptyQueue
		return
	}
	rv = &((*ns).data[0])
	// (*ns).data = (*ns).data[1:len((*ns).data)]
	(*ns).data = (*ns).data[1:]
	return
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (ns *Queue[T]) Truncate() {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	(*ns).data = (*ns).data[:0]
}

/* vim: set noai ts=4 sw=4: */
