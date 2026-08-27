/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package sll_ts implements a generic singly linked list (SLL) that is
// safe for concurrent use.  It is the thread-safe twin of
// github.com/pschlump/pluto/sll — the same API, guarded by a
// sync.RWMutex.
//
// The list supports stack-like operations (Push/Pop/Peek at the head),
// queue-like insertion at the tail (InsertAfterTail), value-based search
// and delete, in-place reversal, a cursor-style iterator
// (Front/Next/Done/Value) and a Go 1.23 range-over-func iterator
// (IterateOver).
//
// Elements are stored and returned by value, so element data is never
// boxed into an interface and never unboxed with a type assertion.
// Lists of types that can be compared with == (the builtin comparable
// constraint) are created with NewSll, which compares elements with the
// == operator; lists of any other type — or with field-based equality —
// are created with NewSllFunc, which takes a caller supplied equality
// function.  The equality function is consulted only by Search, Delete
// and DeleteFound.
//
// Concurrency model:
//
//	Reads (Search, Peek, Len, Length, IsEmpty) take the read lock and
//	release it before returning, so they run in parallel with each other.
//	Writes (InsertBeforeHead, InsertAfterTail, Push, Pop, Delete,
//	DeleteFound, Reverse, Truncate) take the write lock.  Delete and
//	DeleteFound hold the write lock across their search, so
//	search-and-delete is atomic.
//	IterateOver operates on a snapshot taken when it is called (one O(n)
//	copy, under the read lock), so it is safe to use concurrently with
//	any list operation — including mutating the list from inside the
//	loop — and it never observes later modifications.
//	The cursor iterator (Front/Current) walks the LIVE list: each
//	iterator method takes the list's read lock for the duration of that
//	call only (plus the iterator's own mutex, so one iterator may be
//	shared between goroutines).  This is race-free, but the iterator
//	observes concurrent modifications as they happen and terminates
//	early if its current element is deleted.  Prefer IterateOver for a
//	stable view.
//	Dump holds the read lock for the whole dump, so the writer must not
//	call methods on the same list.
//
// Errors, not panics, report empty-list and not-found conditions:
// ErrEmptySll and ErrNotFound.  Compare them with errors.Is.
//
// A nil *Sll and the zero value both behave as an empty list for every
// operation except the insert family: searches report not-found, pops
// and peeks return ErrEmptySll, and the iterators visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewSllFunc(nil)                — nil equality function, caught at construction.
//	Insert-family on a nil list    — a nil list cannot store an element.
//	Insert-family on a zero-value list — no equality function; the message names the constructors.
//
// The insert family is InsertBeforeHead, InsertAfterTail and Push.
package sll_ts

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"
)

// SllElement is a node in the singly linked list.
type SllElement[T any] struct {
	next *SllElement[T]
	data T
}

// Sll is a generic, thread-safe singly linked list of T elements.  Use
// NewSll for element types that support ==, or NewSllFunc for a caller
// supplied equality function.  The zero value is an empty list.
type Sll[T any] struct {
	head, tail *SllElement[T]
	length     int
	lock       sync.RWMutex

	// eq reports whether two elements are considered the same.  It is set
	// by the constructors and is the only thing that knows how to compare
	// T — T itself never has to implement an interface.  It is consulted
	// only by Search, Delete and DeleteFound.
	eq func(a, b T) bool
}

// SllIter is a cursor that allows a for loop to walk the list.  It walks
// the LIVE list: each method takes the list's read lock for the duration
// of that call, so it is race-free but observes concurrent modifications
// and terminates early if its current element is deleted.  For a stable
// view prefer IterateOver.
type SllIter[T any] struct {
	cur      *SllElement[T]
	sll      *Sll[T]
	pos      int
	iterLock sync.RWMutex
}

// ErrEmptySll is returned when a Pop or Peek is attempted on an empty list.
var ErrEmptySll = errors.New("empty sll")

// ErrNotFound is returned when a searched-for or deleted value is not in the list.
var ErrNotFound = errors.New("not found in sll")

// -------------------------------------------------------------------------------------------------------

// NewSll creates a new empty SLL for any element type that can be
// compared with the == operator (the builtin comparable constraint: all
// scalars, strings, arrays, pointers and structs of comparable fields).
// Equality testing never boxes an element into an interface.
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
		panic("sll_ts: NewSllFunc called with a nil equality function")
	}
	return &Sll[T]{eq: fx}
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// equal compares a and b.  The caller must hold a lock; the list must
// have been created by one of the constructors if it is non-empty.
func (ns *Sll[T]) equal(a, b T) bool {
	return ns.eq(a, b)
}

// deleteLocked removes the first element whose data equals t.  The caller
// must hold the write lock.
func (ns *Sll[T]) deleteLocked(t T) (err error) {
	if ns.length == 0 {
		return ErrEmptySll
	}
	var prev *SllElement[T]
	for p := ns.head; p != nil; p = p.next {
		if ns.equal(p.data, t) {
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

// snapshot returns the data of the list in head-to-tail order, taken
// under the read lock.  A nil list yields nil.  The caller must NOT hold
// the lock.
// Complexity is O(n).
func (ns *Sll[T]) snapshot() []T {
	if ns == nil {
		return nil
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	out := make([]T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		out = append(out, p.data)
	}
	return out
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// GetData returns the data stored in this element.
// Complexity is O(1).
func (ee *SllElement[T]) GetData() T {
	return ee.data
}

// Front returns an iterator positioned at the beginning of the list.
func (ns *Sll[T]) Front() *SllIter[T] {
	if ns == nil {
		return &SllIter[T]{}
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return &SllIter[T]{cur: ns.head, sll: ns}
}

// Current takes the node returned from Search
//
//	func (ns *Sll[T]) Search(t T) (rv *SllElement[T], pos int)
//
// and allows you to start an iteration from that point.
func (ns *Sll[T]) Current(el *SllElement[T], pos int) *SllIter[T] {
	return &SllIter[T]{cur: el, sll: ns, pos: pos}
}

// Value returns the current data for this element in the list, or false
// if the iteration is done.
func (iter *SllIter[T]) Value() (rv T, found bool) {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	if iter.sll == nil || iter.cur == nil {
		return
	}
	iter.sll.lock.RLock()
	defer iter.sll.lock.RUnlock()
	if iter.cur == nil {
		return
	}
	return iter.cur.data, true
}

// Next advances to the next element in the list.
func (iter *SllIter[T]) Next() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	if iter.sll == nil || iter.cur == nil {
		return
	}
	iter.sll.lock.RLock()
	defer iter.sll.lock.RUnlock()
	iter.cur = iter.cur.next
	iter.pos++
}

// Done returns true if the end of the list has been reached.
func (iter *SllIter[T]) Done() bool {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	return iter.cur == nil
}

// Pos returns the current "index" of the element being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *SllIter[T]) Pos() int {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	return iter.pos
}

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) IsEmpty() bool {
	if ns == nil {
		return true
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length == 0
}

// InsertBeforeHead will prepend a new node at the head of the list.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
// Complexity is O(1).
func (ns *Sll[T]) InsertBeforeHead(t T) {
	if ns == nil {
		panic("sll_ts: InsertBeforeHead called on a nil list")
	}
	if ns.eq == nil {
		panic("sll_ts: InsertBeforeHead called on a list with no equality function (create the list with NewSll or NewSllFunc)")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
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
		panic("sll_ts: InsertAfterTail called on a nil list")
	}
	if ns.eq == nil {
		panic("sll_ts: InsertAfterTail called on a list with no equality function (create the list with NewSll or NewSllFunc)")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
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
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// Length returns the number of elements in the list.
// Complexity is O(1).
func (ns *Sll[T]) Length() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// Pop will remove the head element from the list.  ErrEmptySll is
// returned if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Pop() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptySll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if ns.length == 0 {
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
// ErrNotFound is returned if no such element exists — including when the
// list is empty, matching the plain sll package.  The write lock is held
// across the search, so search-and-delete is atomic.
// The probe only needs the fields that the equality function reads.
// Complexity is O(n).
func (ns *Sll[T]) Delete(t T) (err error) {
	if ns == nil || ns.IsEmpty() {
		return ErrNotFound
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	return ns.deleteLocked(t)
}

// DeleteFound removes the first element whose data equals the data of the
// element t (typically obtained from Search).  Note that this is a
// value-based re-search, not a direct splice: the first element equal to
// t's data is removed, in O(n).  The write lock is held across the
// search.
// ErrEmptySll is returned for an empty list and ErrNotFound for a nil
// element or no match — the same precedence as the plain sll package.
// Complexity is O(n).
func (ns *Sll[T]) DeleteFound(t *SllElement[T]) (err error) {
	if ns == nil || ns.IsEmpty() {
		return ErrEmptySll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if t == nil {
		return ErrNotFound
	}
	return ns.deleteLocked(t.data)
}

// Search returns the first element whose data equals t, and its position.
// Search is from head to tail.  If not found, it returns (nil, -1).
// The probe only needs the fields that the equality function reads.
//
// Note: the returned element is no longer protected by the list lock once
// Search returns; treat it as read-only.
// Complexity is O(n).
func (ns *Sll[T]) Search(t T) (rv *SllElement[T], pos int) {
	if ns == nil {
		return nil, -1
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
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
	if ns == nil {
		return rv, ErrEmptySll
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
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
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Dump prints out the list, one element per line.  The read lock is held
// for the whole dump, so the writer must not call methods on the same
// list.
// Complexity is O(n).
func (ns *Sll[T]) Dump(fp io.Writer) {
	if ns == nil {
		return
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
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
	ns.lock.Lock()
	defer ns.lock.Unlock()

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
// The iterator operates on a snapshot taken when IterateOver is called,
// so it is safe to call other list operations — including from inside
// the loop — and it never observes later modifications.
// Complexity is O(n) for a full traversal, O(n) extra storage for the
// snapshot.
func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	data := ns.snapshot()
	return func(yield func(int, T) bool) {
		for i, v := range data {
			if !yield(i, v) {
				return
			}
		}
	}
}
