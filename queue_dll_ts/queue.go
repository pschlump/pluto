package queue_dll_ts

/*
Copyright (C) Philip Schlump, 2012-2023.

BSD 3 Clause Licensed.

Basic operations on a Queue

Queue is built with a generic DLL.  The underlying generic type is thread safe.

This is the thread safe implementation.

*	Enqueue() — Inserts an element to the end of the queue (Same as "Push")						O(1)
*	Dequeue() — Removes an element from the start of the queue (Same as "Peek" then "Pop")		O(1)
*	IsEmpty() — Returns true if the queue is empty												O(1)
*	Top() — Returns the first element of the queue (Same as "Peek")								O(1)
*	Push() - Insert into the tail of the Queue.													O(1)
*	Enqueue() - Insert into the tail of the Queue.  Same as Push()								O(1)
* 	Truncate - Delete all the nodes in list. 													O(1)

*/

import (
	"errors"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dllts"
)

// Queue is a generic type buildt on top of a generic DLL
type Queue[T comparable.Equality] struct {
	data dllts.Dll[T]
}

// IsEmpty will return true if the queue is empty
func (ns *Queue[T]) IsEmpty() bool {
	return (*ns).data.Length() == 0
}

// Push will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Push(t *T) {
	(*ns).data.AppendAtTail(t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Enqueue(t *T) {
	(*ns).data.AppendAtTail(t)
}

// An error to indicate that the queue is empty
var ErrEmptyQueue = errors.New("Empty Queue")

// Pop will remove the top element from the queue.  An error is returned if the queue is empty.
func (ns *Queue[T]) Pop() (err error) {
	if ns.IsEmpty() {
		return ErrEmptyQueue
	}
	_, err = (*ns).data.Pop()
	return nil
}

// Length returns the number of elements in the queue.
func (ns *Queue[T]) Length() int {
	return (*ns).data.Length()
}

// Peek returns the top element of the queue or an error indicating that the queue is empty.
func (ns *Queue[T]) Peek() (*T, error) {
	if !ns.IsEmpty() {
		return (*ns).data.Peek()
	}
	return nil, ErrEmptyQueue
}

// Dequeue remove and return an element from the queue (if there is one), else return an error.
func (ns *Queue[T]) Dequeue() (rv *T, err error) {
	if ns.IsEmpty() {
		err = ErrEmptyQueue
		return
	}
	return (*ns).data.Pop()
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (ns *Queue[T]) Truncate() {
	(*ns).data.Truncate()
}

/* vim: set noai ts=4 sw=4: */
