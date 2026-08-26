/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stack implements a generic LIFO stack on top of a slice.
//
// The stack is a struct wrapping the slice, matching the shape of every
// other charon container; elements are stored and returned by value —
// Peek returns (T, error) instead of a pointer aliasing the top element
// — and a nil stack is tolerated everywhere except Push.
//
// Basic operations on a stack:
//
//	Push — Inserts an element at the top.  								O(1) amortized
//	Pop — Removes and returns the top element.  ErrEmptyStack is		O(1)
//	      returned if the stack is empty.
//	IsEmpty — Returns true if the stack is empty.						O(1)
//	Len / Length — Returns the number of elements on the stack.			O(1)
//	Peek — Returns the top element without removing it.					O(1)
//	Truncate — Removes all elements.										O(1)
//	All / Backward — Range-over-func iterators over the stack.			O(n)
//
// The element type needs no constraints at all: there is no ordering and
// no equality to supply, and the zero value of Stack is an empty stack
// ready to use — no constructor required.
//
// Errors, not panics, report the empty stack: ErrEmptyStack.  Compare it
// with errors.Is.
//
// A nil *Stack behaves as an empty stack for every operation except
// Push — a nil stack cannot store an element, and that call panics with
// a message naming the method.  This is the package's only panic.
//
// The stack is not safe for concurrent use.
package stack

import (
	"errors"
)

// Stack is a generic LIFO stack built on top of a slice.
//
// The zero value of Stack is an empty stack, ready to use.
type Stack[T any] struct {
	data []T
}

// ErrEmptyStack is returned by Pop and Peek when the stack is empty.
var ErrEmptyStack = errors.New("empty stack")

// IsEmpty will return true if the stack is empty.
// Complexity is O(1).
func (ns *Stack[T]) IsEmpty() bool {
	return ns == nil || len(ns.data) == 0
}

// Push will push new data of type [T any] onto the stack.
// It panics on a nil stack — the package's only panic.
// Complexity is O(1) amortized.
func (ns *Stack[T]) Push(t T) {
	if ns == nil {
		panic("stack: Push called on a nil stack")
	}
	ns.data = append(ns.data, t)
}

// Pop will remove the top element from the stack.  ErrEmptyStack is
// returned if the stack is empty.
//
// The element is returned by value; the stack no longer holds any
// reference to it.
// Complexity is O(1).
func (ns *Stack[T]) Pop() (rv T, err error) {
	if ns.IsEmpty() {
		return rv, ErrEmptyStack
	}
	last := len(ns.data) - 1
	rv = ns.data[last]
	var zero T
	ns.data[last] = zero // clear the slot so the GC can reclaim the popped value
	ns.data = ns.data[:last]
	if len(ns.data) == 0 {
		ns.data = nil // release the backing array on a full drain
	}
	return
}

// Len returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Len() int {
	if ns == nil {
		return 0
	}
	return len(ns.data)
}

// Length returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Length() int {
	if ns == nil {
		return 0
	}
	return len(ns.data)
}

// Peek returns the top element of the stack or ErrEmptyStack indicating
// that the stack is empty.  Sometimes this is referred to as 'Top'.
//
// The element is returned by value; it does not alias the stack's
// internals and cannot be invalidated by a later Push or Pop.
// Complexity is O(1).
func (ns *Stack[T]) Peek() (rv T, err error) {
	if ns.IsEmpty() {
		return rv, ErrEmptyStack
	}
	return ns.data[len(ns.data)-1], nil
}

// Truncate removes all data from the stack and releases the underlying
// storage.
// Complexity is O(1).
func (ns *Stack[T]) Truncate() {
	if ns == nil {
		return
	}
	ns.data = nil
}
