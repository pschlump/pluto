/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stack_ts implements a generic LIFO stack on top of a slice that
// is safe for concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/stack — the same API, guarded by a
// sync.RWMutex.
//
// For a thread-safe stack with stable O(1) memory per push/pop (no slice
// window, no reallocation) see pluto/stack_sll_ts.
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
//	All / Backward — Range-over-func iterators over a snapshot.			O(n)
//
// Concurrency model:
//
//	Reads (Peek, Len, Length, IsEmpty) take the read lock and release it
//	before returning, so they run in parallel with each other.
//	Writes (Push, Pop, Truncate) take the write lock.
//	All and Backward operate on a snapshot copied under the read lock
//	when they are called, so they are safe to use concurrently with any
//	stack operation — including mutating the stack from inside the loop —
//	and never observe later modifications.
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
// Run the tests with -race.
package stack_ts

import (
	"errors"
	"sync"
)

// Stack is a generic LIFO stack built on top of a slice, safe for
// concurrent use.
//
// The zero value of Stack is an empty stack, ready to use.
type Stack[T any] struct {
	data []T
	lock sync.RWMutex
}

// ErrEmptyStack is returned by Pop and Peek when the stack is empty.
var ErrEmptyStack = errors.New("empty stack")

// IsEmpty will return true if the stack is empty.
// Complexity is O(1).
func (ns *Stack[T]) IsEmpty() bool {
	if ns == nil {
		return true
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.noLockIsEmpty()
}

// noLockIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (ns *Stack[T]) noLockIsEmpty() bool {
	return len(ns.data) == 0
}

// Push will push new data of type [T any] onto the stack.
// It panics on a nil stack — the package's only panic.
// Complexity is O(1) amortized.
func (ns *Stack[T]) Push(t T) {
	if ns == nil {
		panic("stack_ts: Push called on a nil stack")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.data = append(ns.data, t)
}

// Pop will remove the top element from the stack.  ErrEmptyStack is
// returned if the stack is empty.
//
// The element is returned by value; the stack no longer holds any
// reference to it.
// Complexity is O(1).
func (ns *Stack[T]) Pop() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyStack
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if ns.noLockIsEmpty() {
		return rv, ErrEmptyStack
	}
	return ns.noLockPop(), nil
}

// noLockPop removes and returns the top element, zeroing the vacated slot
// so that the backing array does not keep the element alive, and
// releasing the backing array entirely when the stack becomes empty.  The
// caller must hold the lock.
func (ns *Stack[T]) noLockPop() T {
	last := len(ns.data) - 1
	rv := ns.data[last]
	var zero T
	ns.data[last] = zero
	ns.data = ns.data[:last]
	if len(ns.data) == 0 {
		ns.data = nil
	}
	return rv
}

// Len returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Len() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return len(ns.data)
}

// Length returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Length() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return len(ns.data)
}

// Peek returns the top element of the stack or ErrEmptyStack indicating
// that the stack is empty.  Sometimes this is referred to as 'Top'.
//
// The element is returned by value: it is an independent copy taken under
// the read lock, not a live view.  The element may of course be popped by
// another goroutine as soon as Peek returns.
// Complexity is O(1).
func (ns *Stack[T]) Peek() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyStack
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.noLockIsEmpty() {
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
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.data = nil
}
