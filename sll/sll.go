/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

// Package sll implements a generic singly linked list (SLL).
//
// The list supports stack-like operations (Push/Pop/Peek at the head),
// queue-like insertion at the tail (InsertAfterTail), value-based search
// and delete, in-place reversal, and both a cursor-style iterator
// (Front/Next/Done/Value) and Go 1.23 range-over-func iterators
// (IterateOver/IteratePtr).
//
// Elements are stored as *T where T must implement comparable.Equality.
//
// This package is NOT safe for concurrent use; see the sll_ts package
// for a mutex-guarded version with the same API.
package sll

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/pschlump/pluto/comparable"
)

// SllElement is a node in the singly linked list.
type SllElement[T comparable.Equality] struct {
	next *SllElement[T]
	data *T
}

// Sll is a generic singly linked list of *T elements.
type Sll[T comparable.Equality] struct {
	head, tail *SllElement[T]
	length     int
}

// SllIter is a cursor that allows a for loop to walk the list.
type SllIter[T comparable.Equality] struct {
	cur *SllElement[T]
	sll *Sll[T]
	pos int
}

// ErrEmptySll is returned when a Pop or Peek is attempted on an empty list.
var ErrEmptySll = errors.New("empty sll")

// ErrNotFound is returned when a searched-for or deleted value is not in the list.
var ErrNotFound = errors.New("not found in sll")

// -------------------------------------------------------------------------------------------------------

// NewSll creates a new empty SLL and returns it.
// Complexity is O(1).
func NewSll[T comparable.Equality]() *Sll[T] {
	return &Sll[T]{}
}

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *SllElement[T]) GetData() *T {
	return ee.data
}

// -------------------------------------------------------------------------------------------------------

// Front returns an iterator positioned at the beginning of the list.
func (ns *Sll[T]) Front() *SllIter[T] {
	return &SllIter[T]{
		cur: ns.head,
		sll: ns,
	}
}

// Current takes the node returned from Search
//
//	func (ns *Sll[T]) Search(t *T) (rv *SllElement[T], pos int)
//
// and allows you to start an iteration from that point.
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

// Pos returns the current "index" of the element being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *SllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) IsEmpty() bool {
	return ns.length == 0
}

// InsertBeforeHead will prepend a new node at the head of the list.
// Complexity is O(1).
func (ns *Sll[T]) InsertBeforeHead(t *T) {
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
// Complexity is O(1).
func (ns *Sll[T]) InsertAfterTail(t *T) {
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
// Complexity is O(1).
func (ns *Sll[T]) Push(t *T) {
	ns.InsertBeforeHead(t)
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (ns *Sll[T]) Length() int {
	return ns.length
}

// Pop will remove the head element from the list.  ErrEmptySll is returned if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Pop() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = ns.head.data
	ns.head = ns.head.next
	ns.length--
	if ns.head == nil {
		ns.tail = nil
	}
	return
}

// Delete removes the first element whose data IsEqual to t.
// ErrNotFound is returned if no such element exists.
// Complexity is O(n).
func (ns *Sll[T]) Delete(t *T) (err error) {
	el, pos := ns.Search(t)
	if pos < 0 {
		return ErrNotFound
	}
	return ns.DeleteFound(el)
}

// DeleteFound removes the first element whose data IsEqual to the data of the
// element t (typically obtained from Search).  ErrEmptySll is returned for an
// empty list and ErrNotFound if no matching element exists.
// Complexity is O(n).
func (ns *Sll[T]) DeleteFound(t *SllElement[T]) (err error) {
	if ns.IsEmpty() {
		return ErrEmptySll
	}
	if t == nil || t.data == nil {
		return ErrNotFound
	}
	var prev *SllElement[T]
	for p := ns.head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t.data) {
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
			p.data = nil
			return nil
		}
		prev = p
	}
	return ErrNotFound
}

// Search returns the first element whose data IsEqual to t, and its position.
// Search is from head to tail.  If not found, it returns (nil, -1).
// Complexity is O(n).
func (ns *Sll[T]) Search(t *T) (rv *SllElement[T], pos int) {
	if t == nil {
		return nil, -1
	}
	i := 0
	for p := ns.head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t) {
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// Peek returns the head element of the list or an error indicating that the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptySll
	}
	rv = ns.head.data
	return
}

// Truncate removes all data from the list.
// Complexity is O(1).
func (ns *Sll[T]) Truncate() {
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Dump prints out the list.
// Complexity is O(n).
func (tt *Sll[T]) Dump(fp io.Writer) {
	i := 0
	for p := tt.head; p != nil; p = p.next {
		_, _ = fmt.Fprintf(fp, "%d: %+v\n", i, *(p.data))
		i++
	}
}

// Reverse efficiently reverses the direction of the list.
// Complexity is O(n) with O(1) extra storage.
func (ns *Sll[T]) Reverse() {
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
// Complexity is O(n) for a full traversal.
func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, p := 0, ns.head; p != nil; i, p = i+1, p.next {
			if !yield(i, *p.data) {
				return
			}
		}
	}
}

// IteratePtr returns an iterator over index/value-pointer pairs in the list (head to tail),
// for use with Go 1.23 range-over-func loops.  The pointers alias the data stored in the list.
//
// Complexity is O(n) for a full traversal.
func (ns *Sll[T]) IteratePtr() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		for i, p := 0, ns.head; p != nil; i, p = i+1, p.next {
			if !yield(i, p.data) {
				return
			}
		}
	}
}
