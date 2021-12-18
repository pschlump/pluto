package queue

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
)

// Queue is a generic type buildt on top of a slice
type Queue[T any] struct {
	data []T
	head int
}

// IsEmpty will return true if the stack is empty
func (ns *Queue[T]) IsEmpty() bool {
	return len((*ns).data) == 0
}

// Push will push new data of type [T any] onto the stack.
func (ns *Queue[T]) Push(t T) {
	(*ns).data = append((*ns).data, t)
}

// An error to indicate that the stack is empty
var ErrEmptyQueue = errors.New("Empty Queue")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Queue[T]) Pop() error {
	if ns.IsEmpty() {
		return ErrEmptyQueue
	}
	(*ns).data = (*ns).data[1:len((*ns).data)]
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
