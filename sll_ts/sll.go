/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

// Package sll_ts implements a generic singly linked list (SLL) that is safe
// for concurrent use.  It is the thread-safe (mutex-guarded) variant of the
// sll package and exposes the same API.
//
// The list supports stack-like operations (Push/Pop/Peek at the head),
// queue-like insertion at the tail (InsertAfterTail), value-based search
// and delete, in-place reversal, and both a cursor-style iterator
// (Front/Next/Done/Value) and Go 1.23 range-over-func iterators
// (IterateOver/IteratePtr).
//
// Elements are stored as *T where T must implement comparable.Equality.
//
// All methods on Sll and SllIter are safe to call from multiple goroutines.
// The range-over-func iterators operate on a snapshot of the list taken when
// iteration begins.
package sll_ts

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// SllElement is a node in the singly linked list.
type SllElement[T comparable.Equality] struct {
	next *SllElement[T]
	data *T
}

// Sll is a generic, thread-safe singly linked list of *T elements.
type Sll[T comparable.Equality] struct {
	head, tail *SllElement[T]
	length     int
	mu         sync.RWMutex
}

// SllIter is a cursor that allows a for loop to walk the list.
type SllIter[T comparable.Equality] struct {
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
	ns.mu.RLock()
	defer ns.mu.RUnlock()
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
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	iter.sll.mu.RLock()
	defer iter.sll.mu.RUnlock()
	if iter.cur != nil {
		return iter.cur.data
	}
	return nil
}

// Next advances to the next element in the list.
func (iter *SllIter[T]) Next() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	iter.sll.mu.RLock()
	defer iter.sll.mu.RUnlock()
	if iter.cur == nil {
		return
	}
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

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) IsEmpty() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.length == 0
}

// InsertBeforeHead will prepend a new node at the head of the list.
// Complexity is O(1).
func (ns *Sll[T]) InsertBeforeHead(t *T) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
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
	ns.mu.Lock()
	defer ns.mu.Unlock()
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
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.length
}

// Pop will remove the head element from the list.  ErrEmptySll is returned if the list is empty.
// Complexity is O(1).
func (ns *Sll[T]) Pop() (rv *T, err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.length == 0 {
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
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return ns.deleteLocked(t)
}

// DeleteFound removes the first element whose data IsEqual to the data of the
// element t (typically obtained from Search).  ErrEmptySll is returned for an
// empty list and ErrNotFound if no matching element exists.
// Complexity is O(n).
func (ns *Sll[T]) DeleteFound(t *SllElement[T]) (err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if t == nil || t.data == nil {
		return ErrNotFound
	}
	return ns.deleteLocked(t.data)
}

// deleteLocked removes the first element whose data IsEqual to t.
// The caller must hold ns.mu.
func (ns *Sll[T]) deleteLocked(t *T) (err error) {
	if ns.length == 0 {
		return ErrEmptySll
	}
	if t == nil {
		return ErrNotFound
	}
	var prev *SllElement[T]
	for p := ns.head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t) {
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
//
// Note: the returned element is no longer protected by the list lock once
// Search returns; treat it as read-only.
func (ns *Sll[T]) Search(t *T) (rv *SllElement[T], pos int) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
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
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, ErrEmptySll
	}
	rv = ns.head.data
	return
}

// Truncate removes all data from the list.
// Complexity is O(1).
func (ns *Sll[T]) Truncate() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Dump prints out the list.
// Complexity is O(n).
func (ns *Sll[T]) Dump(fp io.Writer) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	i := 0
	for p := ns.head; p != nil; p = p.next {
		_, _ = fmt.Fprintf(fp, "%d: %+v\n", i, *(p.data))
		i++
	}
}

// Reverse efficiently reverses the direction of the list.
// Complexity is O(n) with O(1) extra storage.
func (ns *Sll[T]) Reverse() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

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
// The iterator operates on a snapshot of the list taken when iteration begins,
// so it is safe to mutate the list from inside the loop body.
// Complexity is O(n) for a full traversal, O(n) extra storage for the snapshot.
func (ns *Sll[T]) IterateOver() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range ns.snapshot() {
			if !yield(i, *v) {
				return
			}
		}
	}
}

// IteratePtr returns an iterator over index/value-pointer pairs in the list (head to tail),
// for use with Go 1.23 range-over-func loops.  The pointers alias the data stored in the list.
//
// The iterator operates on a snapshot of the list taken when iteration begins,
// so it is safe to mutate the list from inside the loop body.
// Complexity is O(n) for a full traversal, O(n) extra storage for the snapshot.
func (ns *Sll[T]) IteratePtr() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		for i, v := range ns.snapshot() {
			if !yield(i, v) {
				return
			}
		}
	}
}

// snapshot returns the data pointers of the list in head-to-tail order.
func (ns *Sll[T]) snapshot() []*T {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	out := make([]*T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		out = append(out, p.data)
	}
	return out
}
