/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stack_sll_ts implements a generic, thread-safe LIFO stack on
// top of a singly linked list.
//
// The package is self-contained rather than a wrapper over the sll_ts
// package: pluto's sll_ts requires an equality function at insert (its
// Search needs one), but a stack never compares elements — so wrapping
// would force a dummy comparator and break the constraint-free contract.
// Like every pluto _ts package this one has its own sync.RWMutex and a
// plain singly linked list underneath.
//
// Basic operations on a stack:
//
//	Push — Inserts an element at the top.  								O(1)
//	Pop — Removes and returns the top element.  ErrEmptyStack is		O(1)
//	      returned if the stack is empty.
//	IsEmpty — Returns true if the stack is empty.						O(1)
//	Len / Length — Returns the number of elements on the stack.			O(1)
//	Peek — Returns the top element without removing it.					O(1)
//	Truncate — Removes all elements.										O(1)
//	All / Backward — Range-over-func iterators over a snapshot.			O(n)
//
// Because the stack is built on a linked list (not a slice window like
// the stack package), pushes and pops never reallocate: memory use is
// stable across arbitrary push/pop patterns, at the cost of one small
// node allocation per push.
//
// Concurrency model:
//
//	Reads (Peek, Len, Length, IsEmpty) take the read lock and release it
//	before returning, so they run in parallel with each other.
//	Writes (Push, Pop, Truncate) take the write lock.
//	All and Backward operate on a snapshot taken when they are called
//	(one O(n) copy, under the read lock), so they are safe to use
//	concurrently with any stack operation — including mutating the stack
//	from inside the loop — and never observe later modifications.
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
package stack_sll_ts

import (
	"errors"
	"sync"
)

// stackElement is a node in the singly linked list.
type stackElement[T any] struct {
	data T
	next *stackElement[T]
}

// Stack is a generic, thread-safe LIFO stack built on top of a singly
// linked list.
//
// The zero value of Stack is an empty stack, ready to use.
type Stack[T any] struct {
	head   *stackElement[T]
	length int
	lock   sync.RWMutex
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
	return ns.length == 0
}

// Push will push new data of type [T any] onto the stack.
// It panics on a nil stack — the package's only panic.
// Complexity is O(1).
func (ns *Stack[T]) Push(t T) {
	if ns == nil {
		panic("stack_sll_ts: Push called on a nil stack")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.head = &stackElement[T]{data: t, next: ns.head}
	ns.length++
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
	if ns.length == 0 {
		return rv, ErrEmptyStack
	}
	rv = ns.head.data
	ns.head = ns.head.next
	ns.length--
	return
}

// Len returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Len() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// Length returns the number of elements on the stack.
// Complexity is O(1).
func (ns *Stack[T]) Length() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// Peek returns the top element of the stack or ErrEmptyStack indicating
// that the stack is empty.  Sometimes this is referred to as 'Top'.
//
// The element is returned by value; it does not alias the stack's
// internals and cannot be invalidated by a later Push or Pop.
// Complexity is O(1).
func (ns *Stack[T]) Peek() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyStack
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return rv, ErrEmptyStack
	}
	return ns.head.data, nil
}

// Truncate removes all data from the stack.
// Complexity is O(1).
func (ns *Stack[T]) Truncate() {
	if ns == nil {
		return
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.head = nil
	ns.length = 0
}

// snapshot returns the data of the stack from top to bottom, taken under
// the read lock.  A nil stack yields nil.  The caller must NOT hold the
// lock.
// Complexity is O(n).
func (ns *Stack[T]) snapshot() []T {
	if ns == nil {
		return nil
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	items := make([]T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return items
}
