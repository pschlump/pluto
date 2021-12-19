package stack

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"
)

/*
Basic operations on a stack:

	Push — Inserts an element at the top

	Pop - will remove the top element from the stack.  An error is returned if the stack is empty.

	IsEmpty — Returns true if the stack is empty

	Peek — Returns the top element without removing from the stack
*/

// Stack is a generic type buildt on top of a slice
type Stack[T any] []T

// IsEmpty will return true if the stack is empty
func (ns Stack[T]) IsEmpty() bool {
	return len(ns) == 0
}

// Push will push new data of type [T any] onto the stack.
func (ns *Stack[T]) Push(t T) {
	*ns = append(*ns, t)
}

// An error to indicate that the stack is empty
var ErrEmptyStack = errors.New("Empty Stack")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Stack[T]) Pop() error {
	if ns.IsEmpty() {
		return ErrEmptyStack
	}
	(*ns) = (*ns)[0:len((*ns))-1]
	return nil
}

// Length returns the number of elements in the stack.
func (ns Stack[T]) Length() int {
	return len(ns)
}

// Peek returns the top element of the stack or an error indicating that the stack is empty.
// Some times this is refered to a 'Top'
func (ns *Stack[T]) Peek() (*T, error) {
	if !ns.IsEmpty() {
		return &((*ns)[len(*ns)-1]), nil
	} 
	return nil, ErrEmptyStack
}
