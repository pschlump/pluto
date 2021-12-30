package queue

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Basic operations on a Queue

	Enqueue() — Inserts an element to the end of the queue (Same as "Push")
	Dequeue() — Removes an element from the start of the queue (Same as "Peek" then "Pop")
	IsEmpty() — Returns true if the queue is empty
	Top() — Returns the first element of the queue (Same as "Peek")

*/

import (
	"errors"
)

// Queue is a generic type buildt on top of a slice
type Queue[T any] struct {
	data []T
}

// IsEmpty will return true if the stack is empty
func (ns *Queue[T]) IsEmpty() bool {
	return len((*ns).data) == 0
}

// Push will push new data of type [T any] onto the stack.
func (ns *Queue[T]) Push(t T) {
	(*ns).data = append((*ns).data, t)
}

// Enqueue is the same as Push. Enqueue will push new data of type [T any] onto the stack.
func (ns *Queue[T]) Enqueue(t T) {
	(*ns).data = append((*ns).data, t)
}

// An error to indicate that the stack is empty
var ErrEmptyQueue = errors.New("Empty Queue")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Queue[T]) Pop() error {
	if ns.IsEmpty() {
		return ErrEmptyQueue
	}
	// (*ns).data = (*ns).data[1:len((*ns).data)]
	(*ns).data = (*ns).data[1:]
	return nil
}

// Length returns the number of elements in the stack.
func (ns *Queue[T]) Length() int {
	return len((*ns).data)
}

// Peek returns the top element of the stack or an error indicating that the stack is empty.
func (ns *Queue[T]) Peek() (*T, error) {
	if !ns.IsEmpty() {
		return &((*ns).data[0]), nil
	}
	return nil, ErrEmptyQueue
}

// Dequeue remove and return an element from the queue (if there is one), else return an error.
func (ns *Queue[T]) Dequeue() (rv *T, err error) {
	if ns.IsEmpty() {
		err = ErrEmptyQueue
		return
	}
	rv = &((*ns).data[0])
	// (*ns).data = (*ns).data[1:len((*ns).data)]
	(*ns).data = (*ns).data[1:]
	return
}
