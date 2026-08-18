// Package sll implements a generic singly linked list (SLL) with head and
// tail pointers, an old-style Front/Next/Done iterator, and Go 1.23+
// range-over-func iterators (IterateOver, IteratePtr).
//
// This is the "simple" variant: it is not safe for concurrent use.  Each
// value is stored as a pointer to the caller's data (*T).
package sll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"errors"

	"github.com/pschlump/pluto/comparable"
)

// SllElement is a node in the singly linked list.
type SllElement[T comparable.Equality] struct {
	next *SllElement[T]
	data *T
}

// Sll is a generic singly linked list with head and tail pointers.
type Sll[T comparable.Equality] struct {
	head, tail *SllElement[T]
	length     int
}

// SllIter is an iteration type that allows a for loop to walk the list.
type SllIter[T comparable.Equality] struct {
	cur *SllElement[T]
	sll *Sll[T]
	pos int
}

// -------------------------------------------------------------------------------------------------------

// NewSll creates a new empty SLL and returns it.
// Complexity is O(1).
func NewSll[T comparable.Equality]() *Sll[T] {
	return &Sll[T]{}
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the beginning of the list.
func (ns *Sll[T]) Front() *SllIter[T] {
	return &SllIter[T]{
		cur: ns.head,
		sll: ns,
	}
}

// Current takes a node returned from Search, e.g.
//
//	func (ns *Sll[T]) Search( t *T ) (rv *SllElement[T], pos int) {
//
// and returns an iterator starting at that point in the list.
func (ns *Sll[T]) Current(el *SllElement[T], pos int) *SllIter[T] {
	return &SllIter[T]{
		cur: el,
		sll: ns,
		pos: pos,
	}
}

// Value returns the current data for this element in the list.
func (iter *SllIter[T]) Value() *T {
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances to the next element in the list.
func (iter *SllIter[T]) Next() {
	if iter.cur == nil {
		return
	}
	iter.cur = iter.cur.next
	iter.pos++
}

// Done returns true if the end of the list has been reached.
func (iter *SllIter[T]) Done() bool {
	return iter.cur == nil
}

// Pos returns the current "index" of the element being iterated on.  So if
// the list has 3 elements, a, b, c and we start at the head of the list 'a'
// will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *SllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the list is empty.
func (ns *Sll[T]) IsEmpty() bool {
	return ns.length == 0
}

// InsertBeforeHead will prepend a new node at the head of the list.
// Complexity is O(1).
func (ns *Sll[T]) InsertBeforeHead(t *T) {
	x := &SllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = x
		ns.tail = x
	} else {
		x.next = ns.head
		ns.head = x
	}
	ns.length++
}

// InsertAfterTail will append a new node at the end of the list.
// Complexity is O(1).
func (ns *Sll[T]) InsertAfterTail(t *T) {
	x := &SllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = x
		ns.tail = x
	} else {
		ns.tail.next = x
		ns.tail = x
	}
	ns.length++
}

// Push will prepend a new node at the head of the list (stack semantics,
// paired with Pop).  Complexity is O(1).
func (ns *Sll[T]) Push(t *T) {
	ns.InsertBeforeHead(t)
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (ns *Sll[T]) Length() int {
	return ns.length
}

// ErrEmptySll is an error to indicate that the list is empty.
var ErrEmptySll = errors.New("empty Sll")

// ErrNotFound is an error to indicate that an element was not found in the list.
var ErrNotFound = errors.New("element not found in Sll")

// Pop will remove the element at the head of the list and return it.  An
// error is returned if the list is empty.  Complexity is O(1).
func (ns *Sll[T]) Pop() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	h := ns.head
	rv = h.data
	ns.head = h.next
	h.next = nil // unlink so the node (and anything it references) can be GC'd
	ns.length--
	if ns.head == nil {
		ns.tail = nil
	}
	return
}

// Delete removes the element el (found with Search) from the list.
// Complexity is O(n).  An error is returned if the element is not in the list.
func (ns *Sll[T]) Delete(el *SllElement[T]) (err error) {
	if el == nil || ns.IsEmpty() {
		return ErrNotFound
	}
	if ns.head == el {
		ns.head = el.next
		if ns.tail == el {
			ns.tail = nil
		}
	} else {
		prev := ns.head
		for prev != nil && prev.next != el {
			prev = prev.next
		}
		if prev == nil {
			return ErrNotFound
		}
		prev.next = el.next
		if ns.tail == el {
			ns.tail = prev
		}
	}
	el.next = nil // unlink so the node (and anything it references) can be GC'd
	ns.length--
	return nil
}

// Search returns the given element from a linked list.  Search is from head
// to tail.  Complexity is O(n).  Returns (nil, -1) if not found.
func (ns *Sll[T]) Search(t *T) (rv *SllElement[T], pos int) {
	i := 0
	for p := ns.head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// Peek returns the element at the head of the list, or an error indicating
// that the list is empty.  Complexity is O(1).
func (ns *Sll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = ns.head.data
	return
}

// Truncate removes all data from the list.  The entire chain becomes
// unreachable and can be reclaimed by the GC.  Complexity is O(1).
func (ns *Sll[T]) Truncate() {
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}
