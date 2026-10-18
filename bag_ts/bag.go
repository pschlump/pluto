/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bag_ts implements a generic bag — the add-only collection of
// Sedgwick & Wayne, Algorithms, 4th ed. §1.3 — that is safe for
// concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/bag — the same structure and API, guarded
// by a sync.RWMutex.
//
// Like every pluto package it uses value semantics: elements are stored
// and returned by value, never boxed into an interface.
//
// Concurrency model:
//
//	Reads (Len, Length, IsEmpty) take the read lock and release it
//	before returning, so they run in parallel with each other.
//	Writes (Add, Truncate) take the write lock.
//	All and Backward operate on a snapshot collected under the read
//	lock when they are called, so they are safe to use concurrently
//	with any bag operation — including mutating the bag from inside
//	the loop — and never observe later modifications.
//
//	Set-convenience capabilities (capabilities.go):
//
//	Slice — Fresh []T snapshot of the elements, insertion order.		O(n)
//	String / StringFunc — Render the elements: "[a b c]".			O(n)
//	Equal — Same sequence as another bag (ORDER-SENSITIVE).			O(n)
//	Clone — Deep copy of the bag, its own node chain.				O(n)
//
// Slice, StringFunc and Clone snapshot under the read lock (the format
// callback runs outside the lock); Equal snapshots each side separately
// — one lock is held at a time, so two bags can be compared from
// concurrent goroutines without deadlock.  The capability methods are
// total functions — none panics on a nil bag.  Equal is the one
// order-sensitive Equal in pluto (the SAME multiset added in a
// different order is a different bag) and compares elements pairwise
// with Go's == through interface conversion, so a non-comparable
// element type panics only when two elements are actually compared.
//
// The element type needs no constraints at all: there is no ordering
// and no equality to supply, and the zero value of Bag is an empty bag
// ready to use — no constructor required.
//
// A nil *Bag behaves as an empty bag for every operation except Add —
// a nil bag cannot store an element, and that call panics (after the
// nil guard, before the lock is acquired) with a message naming the
// method.  This is the package's only panic.
//
// Run the tests with -race.
package bag_ts

import (
	"sync"
)

// bagElement is a node in the singly linked list.
type bagElement[T any] struct {
	data T
	next *bagElement[T]
}

// Bag is a generic add-only collection (a multiset) built on top of a
// singly linked list, safe for concurrent use.
//
// The zero value of Bag is an empty bag ready to use.
type Bag[T any] struct {
	head   *bagElement[T]
	tail   *bagElement[T]
	length int
	lock   sync.RWMutex
}

// IsEmpty will return true if the bag is empty.
// Complexity is O(1).
func (b *Bag[T]) IsEmpty() bool {
	if b == nil {
		return true
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.noLockIsEmpty()
}

// noLockIsEmpty is IsEmpty without locking; the caller must hold the lock.
func (b *Bag[T]) noLockIsEmpty() bool {
	return b.length == 0
}

// Add adds new data of type [T any] to the bag.  The element is
// appended at the tail, so it becomes the last element All yields.
// It panics on a nil bag — the package's only panic.
// Complexity is O(1).
func (b *Bag[T]) Add(t T) {
	if b == nil {
		panic("bag_ts: Add called on a nil bag")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.noLockAdd(t)
}

// noLockAdd is Add without locking; the caller must hold the write lock.
func (b *Bag[T]) noLockAdd(t T) {
	e := &bagElement[T]{data: t}
	if b.head == nil {
		b.head = e
	} else {
		b.tail.next = e
	}
	b.tail = e
	b.length++
}

// Len returns the number of elements in the bag.
// Complexity is O(1).
func (b *Bag[T]) Len() int {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.length
}

// Length returns the number of elements in the bag.
// Complexity is O(1).
func (b *Bag[T]) Length() int {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.length
}

// Truncate removes all of the data from the bag, releasing the nodes.
// Complexity is O(1).
func (b *Bag[T]) Truncate() {
	if b == nil {
		return
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.head = nil
	b.tail = nil
	b.length = 0
}
