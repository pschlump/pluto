/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

// Package stack implements a generic LIFO stack on top of a slice.
//
// Basic operations on a stack:
//
//	Push — Inserts an element at the top
//	Pop — Removes the top element from the stack.  An error is returned if the stack is empty.
//	IsEmpty — Returns true if the stack is empty
//	Peek — Returns the top element without removing it from the stack
//
// The stack is not safe for concurrent use; use github.com/pschlump/pluto/stack_sll_ts
// for a thread-safe stack.
package stack

import (
	"errors"
)

// Stack is a generic LIFO stack built on top of a slice.
// The zero value is an empty stack, ready to use.
type Stack[T any] []T

// ErrEmptyStack is returned by Pop and Peek when the stack is empty.
var ErrEmptyStack = errors.New("empty stack")

// IsEmpty will return true if the stack is empty.
func (ns Stack[T]) IsEmpty() bool {
	return len(ns) == 0
}

// Push will push new data of type [T any] onto the stack.
func (ns *Stack[T]) Push(t T) {
	*ns = append(*ns, t)
}

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Stack[T]) Pop() (rv T, err error) {
	if ns.IsEmpty() {
		err = ErrEmptyStack
		return
	}
	last := len(*ns) - 1
	rv = (*ns)[last]
	var zero T
	(*ns)[last] = zero // clear the slot so the GC can reclaim the popped value
	*ns = (*ns)[:last]
	return
}

// Length returns the number of elements in the stack.
func (ns Stack[T]) Length() int {
	return len(ns)
}

// Peek returns a pointer to the top element of the stack or an error indicating
// that the stack is empty.  Sometimes this is referred to as 'Top'.
//
// The returned pointer aliases the element stored in the stack; mutating the
// stack (Push/Pop/Truncate) may invalidate it.
func (ns *Stack[T]) Peek() (*T, error) {
	if !ns.IsEmpty() {
		return &((*ns)[len(*ns)-1]), nil
	}
	return nil, ErrEmptyStack
}

// Truncate removes all data from the stack and releases the underlying storage.
func (ns *Stack[T]) Truncate() {
	*ns = nil
}
