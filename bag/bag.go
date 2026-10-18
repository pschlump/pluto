/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bag implements a generic bag — the add-only collection of
// Sedgwick & Wayne, Algorithms, 4th ed. §1.3 (Bag.java): a container
// whose whole job is to collect elements and iterate them.  There is no
// per-element removal — taking elements back out in an order is a stack
// (LIFO) or a queue (FIFO) — only Truncate, which drops everything.
//
// Like queue, queue_dll and stack this package is constraint-free and
// constructor-free: there is no ordering and no equality to supply
// (nothing in a bag ever compares two elements), so T is any type and
// the zero value of Bag is an empty bag ready to use.  Elements are
// stored and returned by value; no element is ever boxed into an
// interface.
//
// The bag is a self-contained singly linked list with head and tail
// pointers (the queue_dll/dqueue rationale — pluto's sll requires an
// equality function at construction, and a bag never compares).  Add
// appends at the tail, so All iterates in insertion order: element 0 is
// the first element added.  algs4's LinkedBag prepends and therefore
// iterates in reverse-insertion order — an artifact of its
// implementation, not of the abstraction (algs4's own ResizingArrayBag
// iterates in insertion order); this package picks the friendlier
// convention and pins it as a contract.
//
// Operations:
//
//	Add() — Adds an element to the bag.										O(1)
//	IsEmpty() — Returns true if the bag is empty.							O(1)
//	Len / Length() — Returns the number of elements in the bag.				O(1)
//	Truncate() — Removes all elements from the bag.							O(1)
//	All()/Backward() — Range-over-func iterators over the live list.		O(n)
//
//	Set-convenience capabilities (capabilities.go):
//
//	Slice — Fresh []T of the elements, insertion order.				O(n)
//	String / StringFunc — Render the elements: "[a b c]".			O(n)
//	Equal — Same sequence as another bag (ORDER-SENSITIVE).			O(n)
//	Clone — Deep copy of the bag, its own node chain.					O(n)
//
// The capability methods are total functions — none of them panics on a
// nil bag.  Equal is the one order-sensitive Equal in pluto: the SAME
// multiset added in a different order is a different bag ([a b] is not
// [b a]).  It compares elements pairwise with Go's == through interface
// conversion, so a non-comparable element type panics (with the runtime
// comparison panic) only when two elements are actually compared — bags
// of different lengths, and nil or empty bags, are answered without
// touching an element.
//
// A nil *Bag behaves as an empty bag for every operation except Add —
// a nil bag cannot store an element, and that call panics with a
// message naming the method.  This is the package's only panic.
//
// This implementation is NOT thread safe.  For concurrent use see the
// bag_ts twin (../bag_ts) — the same API guarded by a sync.RWMutex.
package bag

// bagElement is a node in the singly linked list.
type bagElement[T any] struct {
	data T
	next *bagElement[T]
}

// Bag is a generic add-only collection (a multiset) built on top of a
// singly linked list.
//
// The zero value of Bag is an empty bag, ready to use.
type Bag[T any] struct {
	head   *bagElement[T]
	tail   *bagElement[T]
	length int
}

// IsEmpty will return true if the bag is empty.
// Complexity is O(1).
func (b *Bag[T]) IsEmpty() bool {
	if b == nil {
		return true
	}
	return b.length == 0
}

// Add adds new data of type [T any] to the bag.  The element is
// appended at the tail, so it becomes the last element All yields.
// It panics on a nil bag — the package's only panic.
// Complexity is O(1).
func (b *Bag[T]) Add(t T) {
	if b == nil {
		panic("bag: Add called on a nil bag")
	}
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
	return b.length
}

// Length returns the number of elements in the bag.
// Complexity is O(1).
func (b *Bag[T]) Length() int {
	if b == nil {
		return 0
	}
	return b.length
}

// Truncate removes all of the data from the bag, releasing the nodes.
// Complexity is O(1).
func (b *Bag[T]) Truncate() {
	if b == nil {
		return
	}
	b.head = nil
	b.tail = nil
	b.length = 0
}
