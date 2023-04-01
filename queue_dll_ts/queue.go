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

Note: This is a subset of the operations that happen on the `dll_ts` so you can just use the
doubley linked list (thread safe) instead.

*/

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/dll_ts"
)

// Queue is a generic type buildt on top of a generic DLL
type Queue[T comparable.Equality] struct {
	data dll_ts.Dll[T]
}

// IsEmpty will return true if the queue is empty
func (ns *Queue[T]) IsEmpty() bool {
	return ns.data.Length() == 0
}

// Push will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Push(t *T) {
	ns.data.AppendAtTail(t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the queue.
func (ns *Queue[T]) Enqueue(t *T) {
	ns.data.AppendAtTail(t)
}

// Pop will remove the top element from the queue.  An error is returned if the queue is empty.
func (ns *Queue[T]) Pop() (err error) {
	_, err = ns.data.Pop()
	return
}

// Length returns the number of elements in the queue.
func (ns *Queue[T]) Length() int {
	return ns.data.Length()
}

// Peek returns the top element of the queue or an error indicating that the queue is empty.
func (ns *Queue[T]) Peek() (*T, error) {
	return ns.data.Peek()
}

// Dequeue remove and return an element from the queue (if there is one), else return an error.
func (ns *Queue[T]) Dequeue() (rv *T, err error) {
	return ns.data.Pop()
}

// Truncate removes all data from the tree.
// Complexity is O(1).
func (ns *Queue[T]) Truncate() {
	ns.data.Truncate()
}

/* vim: set noai ts=4 sw=4: */
