/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package dll_ts implements a generic doubly linked list (DLL) with
// head-and-tail pointers that is safe for concurrent use.  It is the
// thread-safe twin of github.com/pschlump/pluto/dll — the same API,
// guarded by a sync.RWMutex.
//
// Elements are stored and returned by value, so element data is never
// boxed into an interface and never unboxed with a type assertion.
// Lists of types that can be compared with == (the builtin comparable
// constraint) are created with NewDll, which compares elements with the
// == operator; lists of any other type — or with field-based equality —
// are created with NewDllFunc, which takes a caller supplied equality
// function.  The equality function is consulted only by Search,
// ReverseSearch and the Delete calls built on them.
//
// Concurrency model:
//
//	Reads (Search, ReverseSearch, Index, IndexFromTail, Peek, PeekTail,
//	Len, Length, IsEmpty) take the read lock and release it before
//	returning, so they run in parallel with each other.
//	Writes (InsertBeforeHead, Push, AppendAtTail, Enqueue, Pop, PopTail,
//	Delete, DeleteSearch, DeleteFound, DeleteAtHead, DeleteAtTail,
//	Reverse, Truncate, Trim, TrimTail) take the write lock.  Delete
//	holds the write lock across its search, so search-and-delete is
//	atomic.
//	Walk and ReverseWalk hold the read lock while the callback runs; the
//	callback must not call back into the list, or the call can deadlock.
//	All and Backward operate on a snapshot taken when they are called
//	(one O(n) copy, under the read lock), so they are safe to use
//	concurrently with any list operation — including mutating the list
//	from inside the loop — and never observe later modifications.
//	Concat snapshots the source under the source's own read lock before
//	taking the destination's write lock, so no two locks are ever held
//	at once and the source may alias the destination.
//	The legacy DllIter (Front/Rear/Current) walks the LIVE list: each
//	iterator method takes the list's read lock for the duration of that
//	call only.  This is race-free, but an iterator observes concurrent
//	modifications as they happen and terminates early if its current
//	element is deleted.  Prefer All/Backward for a stable view.
//	Lock and Unlock expose the write lock for compound operations (for
//	example a search-and-insert that must be atomic).  No other list
//	operation may run while the lock is held.
//
// Errors, not panics, report empty-list, not-found and out-of-range
// conditions: ErrEmptyDll, ErrNotFound, ErrOutOfRange and ErrInternalDll.
// Compare them with errors.Is.
//
// A nil *Dll and the zero value both behave as an empty list for every
// operation except the insert family: searches report not-found, pops and
// peeks return ErrEmptyDll, and the iterators visit nothing.
//
// The package panics in exactly three situations, all programmer errors
// that cannot be handled where they occur:
//
//	NewDllFunc(nil)               — nil equality function, caught at construction.
//	Insert-family on a nil list   — a nil list cannot store an element.
//	Insert-family on a zero-value list — no equality function; the message names the constructors.
//
// The insert family is InsertBeforeHead, Push, AppendAtTail, Enqueue and
// Concat (which appends).
package dll_ts

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"
	"sync"
)

// DllElement is an element in the doubly linked list.
type DllElement[T any] struct {
	data       T
	next, prev *DllElement[T]
}

// Dll is a generic doubly linked list with head and tail pointers that is
// safe for concurrent use.  Use NewDll for element types that support ==,
// or NewDllFunc for a caller supplied equality function.  The zero value
// is an empty list.
type Dll[T any] struct {
	head, tail *DllElement[T]
	length     int
	lock       sync.RWMutex

	// eq reports whether two elements are considered the same.  It is set
	// by the constructors and is the only thing that knows how to compare
	// T — T itself never has to implement an interface.  It is consulted
	// only by Search, ReverseSearch and the Delete calls built on them.
	eq func(a, b T) bool
}

// DllIter is an iteration type that allows a for loop to walk the list in
// either direction.  It walks the LIVE list: each method takes the list's
// read lock for the duration of that call, so it is race-free but
// observes concurrent modifications and terminates early if its current
// element is deleted.  For a stable view prefer All or Backward.
type DllIter[T any] struct {
	cur      *DllElement[T]
	dll      *Dll[T]
	pos      int
	iterLock sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// NewDll creates a new DLL for any element type that can be compared with
// the == operator (the builtin comparable constraint: all scalars,
// strings, arrays, pointers and structs of comparable fields).  Equality
// testing never boxes an element into an interface.
// Complexity is O(1).
func NewDll[T comparable]() *Dll[T] {
	return &Dll[T]{eq: func(a, b T) bool { return a == b }}
}

// NewDllFunc creates a new DLL that compares elements with the caller
// supplied equality function fx.  This lets any type — including types
// that are not comparable with == (slices, maps, funcs) and structs whose
// identity is a single field — be stored without implementing any
// interface.
// Complexity is O(1).
func NewDllFunc[T any](fx func(a, b T) bool) *Dll[T] {
	if fx == nil {
		panic("dll_ts: NewDllFunc called with a nil equality function")
	}
	return &Dll[T]{eq: fx}
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// equal compares a and b.  The caller must hold a lock; the list must
// have been created by one of the constructors if it is non-empty.
func (ns *Dll[T]) equal(a, b T) bool {
	return ns.eq(a, b)
}

// noLockInsertBeforeHead inserts at the head; the caller must hold the
// write lock.
func (ns *Dll[T]) noLockInsertBeforeHead(t T) {
	x := &DllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = x
		ns.tail = x
		ns.length = 1
	} else {
		x.next = ns.head
		ns.head.prev = x
		ns.head = x
		ns.length++
	}
}

// noLockAppendAtTail appends at the tail; the caller must hold the write
// lock.
func (ns *Dll[T]) noLockAppendAtTail(t T) {
	x := &DllElement[T]{data: t} // Create the node
	if ns.tail == nil {
		ns.head = x
		ns.tail = x
		ns.length = 1
	} else {
		x.prev = ns.tail
		ns.tail.next = x
		ns.tail = x
		ns.length++
	}
}

// noLockPop removes the head; the caller must hold the write lock.
func (ns *Dll[T]) noLockPop() (rv T, err error) {
	if ns.length == 0 {
		return rv, ErrEmptyDll
	}
	rm := ns.head
	rv = rm.data
	ns.head = rm.next
	if ns.head != nil {
		ns.head.prev = nil
	} else {
		ns.tail = nil
	}
	rm.next = nil
	rm.prev = nil
	ns.length--
	return
}

// noLockPopTail removes the tail; the caller must hold the write lock.
func (ns *Dll[T]) noLockPopTail() (rv T, err error) {
	if ns.length == 0 {
		return rv, ErrEmptyDll
	}
	rm := ns.tail
	rv = rm.data
	ns.tail = rm.prev
	if ns.tail != nil {
		ns.tail.next = nil
	} else {
		ns.head = nil
	}
	rm.next = nil
	rm.prev = nil
	ns.length--
	return
}

// noLockDelete removes the given element; the caller must hold the write
// lock.  The element must have come from this list.
func (ns *Dll[T]) noLockDelete(it *DllElement[T]) (err error) {
	if it == nil {
		return ErrNotFound
	}
	if ns.head == it && ns.tail == it {
		ns.head = nil
		ns.tail = nil
		ns.length = 0
		it.next = nil
		it.prev = nil
		return
	}
	if ns.head == it && ns.length > 1 {
		_, err = ns.noLockPop()
		return
	}
	if ns.tail == it && ns.length > 1 {
		_, err = ns.noLockPopTail()
		return
	}
	if ns.length > 2 {
		n := it.prev
		p := it.next
		n.next = p
		p.prev = n
		it.next = nil
		it.prev = nil
		ns.length--
		return
	}
	return ErrInternalDll
}

// snapshot returns the data of the list from head to tail, taken under
// the read lock.  A nil list yields nil.  The caller must NOT hold the
// lock.
// Complexity is O(n).
func (ns *Dll[T]) snapshot() []T {
	if ns == nil {
		return nil
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	data := make([]T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		data = append(data, p.data)
	}
	return data
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// GetData returns the data stored in the element.
// Complexity is O(1).
func (ee *DllElement[T]) GetData() T {
	return ee.data
}

// SetData sets the data stored in the element.  Calling it on an element
// that is inside a list while other goroutines use the list is a race;
// it is intended for standalone elements.
// Complexity is O(1).
func (ee *DllElement[T]) SetData(d T) {
	ee.data = d
}

// Front will start at the beginning of a list for iteration over list.
func (ns *Dll[T]) Front() *DllIter[T] {
	if ns == nil {
		return &DllIter[T]{}
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return &DllIter[T]{cur: ns.head, dll: ns}
}

// Rear will start at the end of a list for iteration over list.
func (ns *Dll[T]) Rear() *DllIter[T] {
	if ns == nil {
		return &DllIter[T]{pos: -1}
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return &DllIter[T]{cur: ns.tail, dll: ns, pos: ns.length - 1}
}

// Current will take the node returned from Search or ReverseSearch
//
//	func (ns *Dll[T]) Search( t T ) (rv *DllElement[T], pos int)
//
// and allow you to start an iteration process from that point.
func (ns *Dll[T]) Current(el *DllElement[T], pos int) *DllIter[T] {
	return &DllIter[T]{cur: el, dll: ns, pos: pos}
}

// Value returns the current data for this element in the list, or false
// if the iteration is done.
func (iter *DllIter[T]) Value() (rv T, found bool) {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	if iter.dll == nil || iter.cur == nil {
		return
	}
	iter.dll.lock.RLock()
	defer iter.dll.lock.RUnlock()
	if iter.cur == nil {
		return
	}
	return iter.cur.data, true
}

// Next advances to the next element in the list.
func (iter *DllIter[T]) Next() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	if iter.dll == nil || iter.cur == nil {
		return
	}
	iter.dll.lock.RLock()
	defer iter.dll.lock.RUnlock()
	iter.cur = iter.cur.next
	iter.pos++
}

// Prev moves back to the previous element in the list.
func (iter *DllIter[T]) Prev() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	if iter.dll == nil || iter.cur == nil {
		return
	}
	iter.dll.lock.RLock()
	defer iter.dll.lock.RUnlock()
	iter.cur = iter.cur.prev
	iter.pos--
}

// Done returns true if the end of the list has been reached.
func (iter *DllIter[T]) Done() bool {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	return iter.cur == nil
}

// Pos returns the current "index" of the element being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *DllIter[T]) Pos() int {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	return iter.pos
}

// IsEmpty will return true if the DLL (queue or stack) is empty.
func (ns *Dll[T]) IsEmpty() bool {
	if ns == nil {
		return true
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length == 0
}

// InsertBeforeHead will insert a new node at the head of the list.
// It returns true if the node was inserted.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
func (ns *Dll[T]) InsertBeforeHead(t T) bool {
	if ns == nil {
		panic("dll_ts: InsertBeforeHead called on a nil list")
	}
	if ns.eq == nil {
		panic("dll_ts: InsertBeforeHead called on a list with no equality function (create the list with NewDll or NewDllFunc)")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.noLockInsertBeforeHead(t)
	return true
}

// Push will insert a new node at the head of the list.
// This is just an alias for InsertBeforeHead.
func (ns *Dll[T]) Push(t T) {
	ns.InsertBeforeHead(t)
}

// AppendAtTail will append a new node to the end of the list.
// It returns true if the node was inserted.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
func (ns *Dll[T]) AppendAtTail(t T) bool {
	if ns == nil {
		panic("dll_ts: AppendAtTail called on a nil list")
	}
	if ns.eq == nil {
		panic("dll_ts: AppendAtTail called on a list with no equality function (create the list with NewDll or NewDllFunc)")
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.noLockAppendAtTail(t)
	return true
}

// Enqueue adds an element to the tail of the list so the DLL can be used as a queue.
// This is just an alias for AppendAtTail.
func (ns *Dll[T]) Enqueue(t T) {
	ns.AppendAtTail(t)
}

// Len returns the number of elements in the list.
func (ns *Dll[T]) Len() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	if ns == nil {
		return 0
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	return ns.length
}

// An error to indicate that the DLL is empty
var ErrEmptyDll = errors.New("empty dll")

// ErrInternalDll indicates an internal inconsistency in the DLL.
var ErrInternalDll = errors.New("internal dll error")

// ErrOutOfRange indicates that a subscript is out of range.
var ErrOutOfRange = errors.New("subscript out of range")

// ErrNotFound indicates that a search failed to find the element.
var ErrNotFound = errors.New("not found")

// Pop will remove the top element from the DLL.  ErrEmptyDll is returned
// if the list is empty.
func (ns *Dll[T]) Pop() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyDll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	return ns.noLockPop()
}

// PopTail will remove the bottom element from the DLL.  ErrEmptyDll is
// returned if the list is empty.
func (ns *Dll[T]) PopTail() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyDll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	return ns.noLockPopTail()
}

// Delete removes a matching element, searching from head to tail.
// If the element is not found ErrNotFound is returned.  The write lock is
// held across the search, so search-and-delete is atomic.
func (ns *Dll[T]) Delete(t T) (err error) {
	if ns == nil {
		return ErrNotFound
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()

	for p := ns.head; p != nil; p = p.next {
		if ns.equal(p.data, t) {
			return ns.noLockDelete(p)
		}
	}
	return ErrNotFound
}

// DeleteSearch will search for a node matching the supplied 't' and if a match is found then that
// node will be deleted.  The search is a linear search from the head.  If it is not found then
// ErrNotFound is returned.  This is an alias for Delete.
func (ns *Dll[T]) DeleteSearch(t T) (err error) {
	return ns.Delete(t)
}

// DeleteFound removes a 'found' element from the DLL; the element must
// have come from this list (from Search, ReverseSearch, Index or
// IndexFromTail).
func (ns *Dll[T]) DeleteFound(it *DllElement[T]) (err error) {
	if ns == nil {
		return ErrNotFound
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	return ns.noLockDelete(it)
}

// DeleteAtHead deletes the first element of the list.
func (ns *Dll[T]) DeleteAtHead() (err error) {
	_, err = ns.Pop()
	return
}

// DeleteAtTail deletes the last element of the list.
func (ns *Dll[T]) DeleteAtTail() (err error) {
	_, err = ns.PopTail()
	return
}

// Peek returns the top element of the DLL (like a Stack) or ErrEmptyDll
// indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyDll
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return rv, ErrEmptyDll
	}
	return ns.head.data, nil
}

// PeekTail returns the last element of the DLL (like a Queue) or
// ErrEmptyDll indicating that the queue is empty.
func (ns *Dll[T]) PeekTail() (rv T, err error) {
	if ns == nil {
		return rv, ErrEmptyDll
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return rv, ErrEmptyDll
	}
	return ns.tail.data, nil
}

// Truncate removes all data from the list.  The equality function is
// kept, so the list remains usable and can simply be refilled.
func (ns *Dll[T]) Truncate() {
	if ns == nil {
		return
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
// If the item is not found then a position of -1 is returned.
// The probe only needs the fields that the equality function reads.
func (ns *Dll[T]) Search(t T) (rv *DllElement[T], pos int) {
	if ns == nil {
		return nil, -1
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
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

// ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
// If the item is not found then a position of -1 is returned.
func (ns *Dll[T]) ReverseSearch(t T) (rv *DllElement[T], pos int) {
	if ns == nil {
		return nil, -1
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := ns.length - 1
	for p := ns.tail; p != nil; p = p.prev {
		if ns.equal(p.data, t) {
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

// ApplyFunction is the type of the callback used by Walk and ReverseWalk.
// Returning true STOPS the walk (note: the opposite convention from the
// pluto tree packages) and the current element and its position are
// returned by the walk.  Caller state is captured in a closure, so it
// keeps its static type and is never boxed.
type ApplyFunction[T any] func(pos int, data T) bool

// Walk - Iterate from head to tail of list. 													O(n)
// If fx returns true the walk stops and the current element and its position are returned.
// If the walk completes without fx returning true then nil, -1 is returned.
// The read lock is held while fx runs: fx must not call methods on the
// same list, or the call can deadlock.
func (ns *Dll[T]) Walk(fx ApplyFunction[T]) (rv *DllElement[T], pos int) {
	if ns == nil {
		return nil, -1
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := 0
	for p := ns.head; p != nil; p = p.next {
		if fx(i, p.data) {
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseWalk - Iterate from tail to head of list. 											O(n)
// If fx returns true the walk stops and the current element and its position are returned.
// If the walk completes without fx returning true then nil, -1 is returned.
// The read lock is held while fx runs: fx must not call methods on the
// same list, or the call can deadlock.
func (ns *Dll[T]) ReverseWalk(fx ApplyFunction[T]) (rv *DllElement[T], pos int) {
	if ns == nil {
		return nil, -1
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := ns.length - 1
	for p := ns.tail; p != nil; p = p.prev {
		if fx(i, p.data) {
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

// ReverseList - Reverse all the nodes in list. 												O(n)
func (ns *Dll[T]) ReverseList() {
	ns.Reverse()
}

// Index will return the Nth item from the list, counting from the head.
// ErrOutOfRange is returned for an invalid subscript.
func (ns *Dll[T]) Index(sub int) (rv *DllElement[T], err error) {
	if ns == nil {
		return nil, ErrOutOfRange
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, ErrOutOfRange
	}

	if sub < 0 || sub >= ns.length {
		return nil, ErrOutOfRange
	} else if sub < (ns.length / 2) {
		i := 0
		rv = ns.head
		for ; i < sub; rv = rv.next {
			i++
		}
		return
	} else {
		i := ns.length - 1
		rv = ns.tail
		for ; rv != nil && i > sub; rv = rv.prev {
			i--
		}
		return
	}
}

// IndexFromTail will return the Nth item from the list counting from the tail,
// so IndexFromTail(0) is the last element of the list.
// ErrOutOfRange is returned for an invalid subscript.
func (ns *Dll[T]) IndexFromTail(sub int) (rv *DllElement[T], err error) {
	if ns == nil {
		return nil, ErrOutOfRange
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	if ns.length == 0 {
		return nil, ErrOutOfRange
	}

	if sub < 0 || sub >= ns.length {
		return nil, ErrOutOfRange
	} else if sub < (ns.length / 2) {
		i := 0
		rv = ns.tail
		for ; i < sub; rv = rv.prev {
			i++
		}
		return
	} else {
		i := ns.length - 1
		rv = ns.head
		for ; rv != nil && i > sub; rv = rv.next {
			i--
		}
		return
	}
}

// Trim will cut the list to the specified length, keeping the first n elements.
// The list is unchanged if it is shorter than n.  If n <= 0 the list is emptied.
// ErrEmptyDll is returned if the list is already empty.
// Complexity is O(n).
func (ns *Dll[T]) Trim(n int) (err error) {
	if ns == nil {
		return ErrEmptyDll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()

	if ns.length == 0 {
		return ErrEmptyDll
	}
	if n <= 0 {
		ns.head = nil
		ns.tail = nil
		ns.length = 0
		return nil
	}
	if ns.length <= n {
		return nil
	}
	tmp := ns.head
	for i := 0; i < n-1; i++ {
		tmp = tmp.next
	}
	// Unlink the dropped tail of the list so it can be garbage collected.
	dropped := tmp.next
	tmp.next = nil
	if dropped != nil {
		dropped.prev = nil
	}
	ns.tail = tmp
	ns.length = n
	return nil
}

// TrimTail will cut the list to the specified length, keeping the last n elements.
// The list is unchanged if it is shorter than n.  If n <= 0 the list is emptied.
// ErrEmptyDll is returned if the list is already empty.
// Complexity is O(n).
func (ns *Dll[T]) TrimTail(n int) (err error) {
	if ns == nil {
		return ErrEmptyDll
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()

	if ns.length == 0 {
		return ErrEmptyDll
	}
	if n <= 0 {
		ns.head = nil
		ns.tail = nil
		ns.length = 0
		return nil
	}
	if ns.length <= n {
		return nil
	}
	tmp := ns.tail
	for i := 0; i < n-1; i++ {
		tmp = tmp.prev
	}
	// Unlink the dropped head of the list so it can be garbage collected.
	dropped := tmp.prev
	tmp.prev = nil
	if dropped != nil {
		dropped.next = nil
	}
	ns.head = tmp
	ns.length = n
	return nil
}

// Concat appends a copy of the elements of yy to the end of ns.
// yy is unchanged.  If ns == yy the list is duplicated onto itself.
// The source is snapshotted under its own read lock before ns's write
// lock is taken, so no two locks are ever held at once and yy may safely
// alias ns.  Concatenating a nil or empty source is a no-op.
// Concat otherwise follows the insert contract: ns must have been created
// with NewDll or NewDllFunc.
// Complexity is O(len(yy)).
func (ns *Dll[T]) Concat(yy *Dll[T]) {
	// Snapshot the source first: a nil or empty source is a no-op even on
	// a nil list; the insert contract only fires when an element would
	// actually be stored.
	data := yy.snapshot()
	if len(data) == 0 {
		return
	}
	if ns == nil {
		panic("dll_ts: Concat called on a nil list")
	}
	if ns.eq == nil {
		panic("dll_ts: Concat called on a list with no equality function (create the list with NewDll or NewDllFunc)")
	}

	ns.lock.Lock()
	defer ns.lock.Unlock()
	for _, d := range data {
		ns.noLockAppendAtTail(d)
	}
}

// Dump prints the list, one element per line, to fo.  The read lock is
// held for the whole dump, so the writer must not call methods on the
// same list.
func (ns *Dll[T]) Dump(fo io.Writer) {
	if ns == nil {
		return
	}
	ns.lock.RLock()
	defer ns.lock.RUnlock()
	i := 0
	for p := ns.head; p != nil; p = p.next {
		_, _ = fmt.Fprintf(fo, "%d: %+v\n", i, p.data)
		i++
	}
}

// Reverse - efficiently reverse direction on a list.  O(n) with storage O(1)
func (ns *Dll[T]) Reverse() {
	if ns == nil {
		return
	}
	ns.lock.Lock()
	defer ns.lock.Unlock()

	var next *DllElement[T]

	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next, cp.prev = cp.prev, cp.next
	}

	ns.head, ns.tail = ns.tail, ns.head
}

// Lock takes the list's write lock.  It must be paired with a call to
// Unlock.  No other list operation may be performed by any goroutine
// while the lock is held, so prefer the built-in locked operations where
// possible.  Locking a nil list is a no-op.
func (ns *Dll[T]) Lock() {
	if ns == nil {
		return
	}
	ns.lock.Lock()
}

// Unlock releases the list's write lock taken by Lock.  Unlocking a nil
// list is a no-op.
func (ns *Dll[T]) Unlock() {
	if ns == nil {
		return
	}
	ns.lock.Unlock()
}

// -----------------------------------------------------------------------------------------------------------
// Go 1.23+ range-over-func iterators.

// All returns an iterator over the elements of the list from head to tail.
// The index starts at 0 at the head of the list.  The iterator operates
// on a snapshot taken when All is called, so it is safe to call other
// list operations — including from inside the loop — and it never
// observes later modifications.
func (ns *Dll[T]) All() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	data := ns.snapshot()
	return func(yield func(int, T) bool) {
		for i, d := range data {
			if !yield(i, d) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the list from tail to head.
// The index starts at Length()-1 at the tail of the list.  The iterator
// operates on a snapshot taken when Backward is called, so it is safe to
// call other list operations — including from inside the loop — and it
// never observes later modifications.
func (ns *Dll[T]) Backward() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	data := ns.snapshot()
	return func(yield func(int, T) bool) {
		for i, d := range slices.Backward(data) {
			if !yield(i, d) {
				return
			}
		}
	}
}

// IterateOver is a legacy name for All.
func (ns *Dll[T]) IterateOver() iter.Seq2[int, T] {
	return ns.All()
}
