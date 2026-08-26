/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package sll implements a generic singly linked list (SLL).
//
// The list supports stack-like operations (Push/Pop/Peek at the head),
// queue-like insertion at the tail (InsertAfterTail), value-based search
// and delete, in-place reversal, and both a cursor-style iterator
// (Front/Next/Done/Value) and a Go 1.23 range-over-func iterator
// (IterateOver).
//
// It is a rework of github.com/pschlump/pluto/sll in which the
// comparable.Equality interface constraint has been replaced with plain
// Go type parameters.  Elements are stored and returned by value, so
// element data is never boxed into an interface and never unboxed with a
// type assertion.  Lists of types that can be compared with == (the
// builtin comparable constraint — all scalars, strings, arrays, and
// structs of comparable fields) are created with NewSll, which compares
// elements with the == operator; lists of any other type — or with
// field-based equality — are created with NewSllFunc, which takes a
// caller supplied equality function; the element type does not have to
// implement any interface.  The equality function is consulted only by
// Search, Delete and DeleteFound; the stack and positional operations
// never compare elements.
//
// Operations:
//
//	InsertBeforeHead / Push — prepend at the head.  										O(1)
//	InsertAfterTail — append at the tail (queue-style insertion).						O(1)
//	Pop — remove and return the head element.											O(1)
//	Peek — look at the head element.														O(1)
//	Search — find the first element equal to a probe, head to tail.						O(n)
//	Delete — remove the first element equal to a probe.									O(n)
//	DeleteFound — remove the first element equal to a found element's data.				O(n)
//	Reverse — reverse the list in place.													O(n), O(1) storage
//	Truncate — remove all elements.														O(1)
//	IsEmpty / Len / Length — size queries.												O(1)
//	Dump — print the list, one element per line.											O(n)
//	Front/Current + Next/Done/Pos/Value — cursor iterator.								O(n) total
//	IterateOver — iter.Seq2[int, T], head to tail.										O(n)
//
// Errors, not panics, report empty-list and not-found conditions:
// ErrEmptySll and ErrNotFound.  Compare them with errors.Is.
//
// A nil *Sll and the zero value both behave as an empty list for every
// operation except the insert family: searches report not-found, pops and
// peeks return ErrEmptySll, and the iterators visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewSllFunc(nil)                — nil equality function, caught at construction.
//	Insert-family on a nil list    — a nil list cannot store an element.
//	Insert-family on a zero-value list — no equality function; the message names the constructors.
//
// The insert family is InsertBeforeHead, InsertAfterTail and Push.
//
// This package is NOT safe for concurrent use; a mutex guarded
// thread-safe twin has the exact same interface.
package sll

import (
	"errors"
	"fmt"
	"io"
	"iter"
)

// SllElement is a node in the singly linked list.
type SllElement[T any] struct {
	next *SllElement[T]
	data T
}

// Sll is a generic singly linked list of T elements.  Use NewSll for
// element types that support ==, or NewSllFunc for a caller supplied
// equality function.  The zero value is an empty list.
type Sll[T any] struct {
	head, tail *SllElement[T]
	length     int

	// eq reports whether two elements are considered the same.  It is set
	// by the constructors and is the only thing that knows how to compare
	// T — T itself never has to implement an interface.  It is consulted
	// only by Search, Delete and DeleteFound.
	eq func(a, b T) bool
}

// SllIter is a cursor that allows a for loop to walk the list.
type SllIter[T any] struct {
	cur *SllElement[T]
	pos int
}

// ErrEmptySll is returned when a Pop or Peek is attempted on an empty list.
var ErrEmptySll = errors.New("empty sll")

// ErrNotFound is returned when a searched-for or deleted value is not in the list.
var ErrNotFound = errors.New("not found in sll")

// -------------------------------------------------------------------------------------------------------

// NewSll creates a new empty SLL for any element type that can be compared
// with the == operator (the builtin comparable constraint: all scalars,
// strings, arrays, pointers and structs of comparable fields).  Equality
// testing never boxes an element into an interface.
// Complexity is O(1).
func NewSll[T comparable]() *Sll[T] {
	return &Sll[T]{eq: func(a, b T) bool { return a == b }}
}

// NewSllFunc creates a new empty SLL that compares elements with the
// caller supplied equality function fx.  This lets any type — including
// types that are not comparable with == (slices, maps, funcs) and structs
// whose identity is a single field — be stored without implementing any
// interface.
// Complexity is O(1).
func NewSllFunc[T any](fx func(a, b T) bool) *Sll[T] {
	if fx == nil {
		panic("sll: NewSllFunc called with a nil equality function")
	}
	return &Sll[T]{eq: fx}
}

// equal compares a and b, guarding against a zero-value list that was not
// created by one of the constructors.
func (ns *Sll[T]) equal(a, b T) bool {
	if ns.eq == nil {
		panic("sll: no equality function (create the list with NewSll or NewSllFunc)")
	}
	return ns.eq(a, b)
}

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *SllElement[T]) GetData() T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the beginning of the list.
func (ns *Sll[T]) Front() *SllIter[T] {
	if ns == nil {
		return &SllIter[T]{}
	}
	return &SllIter[T]{cur: ns.head}
}

// Current takes the node returned from Search
//
//	func (ns *Sll[T]) Search(t T) (rv *SllElement[T], pos int)
//
// and allows you to start an iteration from that point.
func (ns *Sll[T]) Current(el *SllElement[T], pos int) *SllIter[T] {
	return &SllIter[T]{cur: el, pos: pos}
}

// Value returns the current data for this element in the list, or false
// if the iteration is done.
func (iter *SllIter[T]) Value() (rv T, found bool) {
	if iter.cur != nil {
		return iter.cur.data, true
	}
	return
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

// Pos returns the current "index" of the element being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *SllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) IsEmpty() bool {
	return ns == nil || ns.length == 0
}

// InsertBeforeHead will prepend a new node at the head of the list.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
// Complexity is O(1).
func (ns *Sll[T]) InsertBeforeHead(t T) {
	if ns == nil {
		panic("sll: InsertBeforeHead called on a nil list")
	}
	if ns.eq == nil {
		panic("sll: InsertBeforeHead called on a list with no equality function (create the list with NewSll or NewSllFunc)")
	}
	x := &SllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.tail = x
	} else {
		x.next = ns.head
	}
	ns.head = x
	ns.length++
}

// InsertAfterTail will append a new node at the end of the list.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
// Complexity is O(1).
func (ns *Sll[T]) InsertAfterTail(t T) {
	if ns == nil {
		panic("sll: InsertAfterTail called on a nil list")
	}
	if ns.eq == nil {
		panic("sll: InsertAfterTail called on a list with no equality function (create the list with NewSll or NewSllFunc)")
	}
	x := &SllElement[T]{data: t} // Create the node
	if ns.tail == nil {
		ns.head = x
	} else {
		ns.tail.next = x
	}
	ns.tail = x
	ns.length++
}

// Push will prepend a new node at the head of the list (stack semantics).
// This is just an alias for InsertBeforeHead.
// Complexity is O(1).
func (ns *Sll[T]) Push(t T) {
	ns.InsertBeforeHead(t)
}

// Len returns the number of elements in the list.
// Complexity is O(1).
func (ns *Sll[T]) Len() int {
	if ns == nil {
		return 0
	}
	return ns.length
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (ns *Sll[T]) Length() int {
	if ns == nil {
		return 0
	}
	return ns.length
}

// Pop will remove the head element from the list.  ErrEmptySll is
// returned if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Pop() (rv T, err error) {
	if ns.IsEmpty() {
		return rv, ErrEmptySll
	}
	rv = ns.head.data
	ns.head = ns.head.next
	ns.length--
	if ns.head == nil {
		ns.tail = nil
	}
	return
}

// Delete removes the first element whose data equals t.
// ErrNotFound is returned if no such element exists.
// The probe only needs the fields that the equality function reads.
// Complexity is O(n).
func (ns *Sll[T]) Delete(t T) (err error) {
	el, pos := ns.Search(t)
	if pos < 0 {
		return ErrNotFound
	}
	return ns.DeleteFound(el)
}

// DeleteFound removes the first element whose data equals the data of the
// element t (typically obtained from Search).  Note that — unlike the dll
// package — this is a value-based re-search, not a direct splice: the
// first element equal to t's data is removed, in O(n).
// ErrEmptySll is returned for an empty list and ErrNotFound for a nil
// element or no match.
// Complexity is O(n).
func (ns *Sll[T]) DeleteFound(t *SllElement[T]) (err error) {
	if ns.IsEmpty() {
		return ErrEmptySll
	}
	if t == nil {
		return ErrNotFound
	}
	var prev *SllElement[T]
	for p := ns.head; p != nil; p = p.next {
		if ns.equal(p.data, t.data) {
			if prev == nil {
				ns.head = p.next
			} else {
				prev.next = p.next
			}
			if ns.tail == p {
				ns.tail = prev
			}
			ns.length--
			p.next = nil // unlink so the GC can reclaim the node
			return nil
		}
		prev = p
	}
	return ErrNotFound
}

// Search returns the first element whose data equals t, and its position.
// Search is from head to tail.  If not found, it returns (nil, -1).
// The probe only needs the fields that the equality function reads.
// Complexity is O(n).
func (ns *Sll[T]) Search(t T) (rv *SllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1
	}
	i := 0
	for p := ns.head; p != nil; p = p.next {
		if ns.equal(p.data, t) {
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// Peek returns the head element of the list or ErrEmptySll indicating
// that the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Peek() (rv T, err error) {
	if ns.IsEmpty() {
		return rv, ErrEmptySll
	}
	return ns.head.data, nil
}

// Truncate removes all data from the list.  The equality function is
// kept, so the list remains usable and can simply be refilled.
// Complexity is O(1).
func (ns *Sll[T]) Truncate() {
	if ns == nil {
		return
	}
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Dump prints out the list, one element per line.
// Complexity is O(n).
func (ns *Sll[T]) Dump(fp io.Writer) {
	if ns == nil {
		return
	}
	i := 0
	for p := ns.head; p != nil; p = p.next {
		_, _ = fmt.Fprintf(fp, "%d: %+v\n", i, p.data)
		i++
	}
}

// Reverse efficiently reverses the direction of the list.
// Complexity is O(n) with O(1) extra storage.
func (ns *Sll[T]) Reverse() {
	if ns == nil {
		return
	}
	var prev, next *SllElement[T]
	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next = prev
		prev = cp
	}
	ns.head, ns.tail = ns.tail, ns.head
}

// IterateOver returns an iterator over index/value pairs in the list (head to tail),
// for use with Go 1.23 range-over-func loops:
//
//	for i, v := range list.IterateOver() { ... }
//
// The list must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n) for a full traversal.
func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i, p := 0, ns.head; p != nil; i, p = i+1, p.next {
			if !yield(i, p.data) {
				return
			}
		}
	}
}
