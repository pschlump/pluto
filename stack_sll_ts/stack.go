/*
Copyright (C) Philip Schlump, 2023.

BSD 3 Clause Licensed.
*/

// Package stack implements a generic, thread-safe LIFO stack on top of the
// thread-safe singly linked list github.com/pschlump/pluto/sll_ts.
//
// Basic operations on a stack:
//
//	Push — Inserts an element at the top
//	Pop — Removes the top element from the stack.  An error is returned if the stack is empty.
//	IsEmpty — Returns true if the stack is empty
//	Peek — Returns the top element without removing it from the stack
//
// All operations are guarded by a mutex (inherited from sll_ts.Sll) and are
// safe for concurrent use.
//
// Note: This is a subset of the operations that happen on the `sll_ts` so you
// can just use the singly linked list (thread safe) instead.
package stack

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/sll_ts"
)

// Stack is a generic, thread-safe LIFO stack built on top of a singly
// linked list.  Elements are stored and returned by pointer.
// The zero value is an empty stack, ready to use.
type Stack[T comparable.Equality] struct {
	data sll_ts.Sll[T]
}

// IsEmpty will return true if the stack is empty.
func (ns *Stack[T]) IsEmpty() bool {
	return ns.data.IsEmpty()
}

// Push will push new data of type [T comparable.Equality] onto the stack.
func (ns *Stack[T]) Push(t *T) {
	ns.data.Push(t)
}

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Stack[T]) Pop() (rv *T, err error) {
	return ns.data.Pop()
}

// Length returns the number of elements in the stack.
func (ns *Stack[T]) Length() int {
	return ns.data.Length()
}

// Peek returns the top element of the stack or an error indicating that the stack is empty.
// Sometimes this is referred to as 'Top'.
func (ns *Stack[T]) Peek() (*T, error) {
	return ns.data.Peek()
}

// Truncate removes all data from the stack.
func (ns *Stack[T]) Truncate() {
	ns.data.Truncate()
}

/* vim: set noai ts=4 sw=4: */
