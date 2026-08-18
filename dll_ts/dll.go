package dll_ts

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.

Package dll_ts implements a generic doubly linked list (DLL) with head-and-tail
pointers.  It is the thread-safe (goroutine-safe) version of ../dll: every
operation is guarded by a sync.RWMutex.  It has the exact same interface as
the dll package.

*	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
*	Delete — Deletes a specified element from the linked list (element can be found via Search). O(n)
*	DeleteFound — Deletes a previously found element (from Search/ReverseSearch/Index).		O(1)
*	DeleteSearch — Deletes a specified element, searching from head to tail.					O(n)
*	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
*	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
*	Index — Return the Nth item in the list - in a format usable with DeleteFound.				O(n) n/2
*	IndexFromTail — Return the Nth item from the tail of the list.								O(n) n/2
*	InsertBeforeHead — Inserts a new element before the current first element of list.  		O(1)
*	IsEmpty — Returns true if the linked list is empty											O(1)
*	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
*	Peek — Look at data at head of list.														O(1)
*	PeekTail — Look at data at tail of list.													O(1)
*	Pop	— Remove and return from the head of the list.											O(1)
*	PopTail — Remove and return from the tail of the list.										O(1)
*	Push — Insert at the head of the list.														O(1)
*	ReverseList — Reverse all the nodes in list. 												O(n)
*	Reverse — Reverse all the nodes in list with O(1) extra storage.							O(n)
*	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
*	ReverseWalk — Iterate from tail to head of list. 											O(n)
*	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n) n/2
*	Truncate — Delete all the nodes in list. 													O(1)
*	Walk — Iterate from head to tail of list. 													O(n)
*	Trim — Cut list to specified length, keeping the head; unchanged if shorter.				O(n)
*	TrimTail — Cut list to specified length, keeping the tail; unchanged if shorter.			O(n)
*	Concat — Append a copy of the elements of another list to this list.						O(n)

With the basic stack operations it also can be used as a stack:

*	Push — Inserts an element at the top														O(1)
*	Pop — Will remove the top element from the stack.  An error is returned if the stack is		O(1)
		empty.
*	IsEmpty — Returns true if the stack is empty												O(1)
*	Peek — Returns the top element without removing from the stack								O(1)

With the use of Enqueue it can be used as a queue.  Enqueue is a synonym for AppendAtTail.	O(1)

*	PeekTail — Peek returns the last element of the DLL (like a queue) or an error 				O(1)
		indicating that the queue is empty.
*	PopTail — Remove the element at the end of the DLL.											O(1)
*	Enqueue — Add to the tail so that DLL can be used as a queue.								O(1)

Iteration is supported by the legacy DllIter type (Front/Rear/Done/Next/Prev/Value),
by the Walk/ReverseWalk callbacks, and by Go 1.23+ range-over-func iterators:

*	All — iter.Seq2[int, T] from head to tail.													O(n)
*	Backward — iter.Seq2[int, T] from tail to head.												O(n)
*	IterateOver — legacy name for All.															O(n)
*	IteratePtr — iter.Seq2[int, *T] from head to tail.											O(n)

The range-over-func iterators operate on a consistent snapshot of the list
taken under a read lock, so they never hold the lock while user code runs.

Note: Walk and ReverseWalk hold a read lock while the user callback runs, so
the callback must not call back into the list (that would deadlock).
*/

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// DllElement is a node in the doubly linked list.
type DllElement[T comparable.Equality] struct {
	next, prev *DllElement[T]
	Data       *T
}

// Dll is a generic doubly linked list with head and tail pointers.
// The zero value is an empty list ready to use.
// All operations are safe for concurrent use by multiple goroutines.
type Dll[T comparable.Equality] struct {
	head, tail *DllElement[T]
	length     int
	mu         sync.RWMutex
}

// DllIter is an iteration type that allows a for loop to walk the list.
type DllIter[T comparable.Equality] struct {
	cur      *DllElement[T]
	dll      *Dll[T]
	pos      int
	iterLock sync.RWMutex
}

// -------------------------------------------------------------------------------------------------------

// NewDll creates a new DLL and returns it.
// Complexity is O(1).
func NewDll[T comparable.Equality]() *Dll[T] {
	return &Dll[T]{}
}

// GetData returns the data stored in the element.
// Complexity is O(1).
func (ee *DllElement[T]) GetData() *T {
	return ee.Data
}

// SetData sets the data stored in the element.
// Complexity is O(1).
func (ee *DllElement[T]) SetData(d *T) {
	ee.Data = d
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the beginning of a list for iteration over list.
func (ns *Dll[T]) Front() *DllIter[T] {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return &DllIter[T]{
		cur: ns.head,
		dll: ns,
	}
}

// Rear will start at the end of a list for iteration over list.
func (ns *Dll[T]) Rear() *DllIter[T] {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return &DllIter[T]{
		cur: ns.tail,
		dll: ns,
		pos: ns.length - 1,
	}
}

// Current will take the node returned from Search or ReverseSearch
//
//	func (ns *Dll[T]) Search( t *T ) (rv *DllElement[T], pos int) {
//
// and allow you to start an iteration process from that point.
func (ns *Dll[T]) Current(el *DllElement[T], pos int) *DllIter[T] {
	return &DllIter[T]{
		cur: el,
		dll: ns,
		pos: pos,
	}
}

// Value returns the current data for this element in the list.
func (iter *DllIter[T]) Value() *T {
	iter.iterLock.RLock()
	defer iter.iterLock.RUnlock()
	iter.dll.mu.RLock()
	defer iter.dll.mu.RUnlock()
	if iter.cur != nil {
		return iter.cur.Data
	}
	return nil
}

// Next advances to the next element in the list.
func (iter *DllIter[T]) Next() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	iter.dll.mu.RLock()
	defer iter.dll.mu.RUnlock()
	if iter.cur == nil {
		return
	}
	iter.cur = iter.cur.next
	iter.pos++
}

// Prev moves back to the previous element in the list.
func (iter *DllIter[T]) Prev() {
	iter.iterLock.Lock()
	defer iter.iterLock.Unlock()
	iter.dll.mu.RLock()
	defer iter.dll.mu.RUnlock()
	if iter.cur == nil {
		return
	}
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

// -------------------------------------------------------------------------------------------------------
// IsEmpty will return true if the DLL (queue or stack) is empty
func (ns *Dll[T]) IsEmpty() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.length == 0
}

func (ns *Dll[T]) noLockInsertBeforeHead(t *T) {
	x := DllElement[T]{Data: t} // Create the node
	if ns.head == nil {
		ns.head = &x
		ns.tail = &x
		ns.length = 1
	} else {
		x.next = ns.head
		ns.head.prev = &x
		ns.head = &x
		ns.length++
	}
}

// InsertBeforeHead will insert a new node at the head of the list.
// It returns true if the node was inserted.
func (ns *Dll[T]) InsertBeforeHead(t *T) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.noLockInsertBeforeHead(t)
	return true
}

// Push will insert a new node at the head of the list.
// This is just an alias for InsertBeforeHead.
func (ns *Dll[T]) Push(t *T) {
	ns.InsertBeforeHead(t)
}

func (ns *Dll[T]) noLockAppendAtTail(t *T) {
	x := DllElement[T]{Data: t} // Create the node
	if ns.tail == nil {
		ns.head = &x
		ns.tail = &x
		ns.length = 1
	} else {
		x.prev = ns.tail
		ns.tail.next = &x
		ns.tail = &x
		ns.length++
	}
}

// AppendAtTail will append a new node to the end of the list.
// It returns true if the node was inserted.
func (ns *Dll[T]) AppendAtTail(t *T) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.noLockAppendAtTail(t)
	return true
}

// Enqueue adds an element to the tail of the list so the DLL can be used as a queue.
// This is just an alias for AppendAtTail.
func (ns *Dll[T]) Enqueue(t *T) {
	ns.AppendAtTail(t)
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.length
}

// An error to indicate that the DLL is empty
var ErrNotFound = errors.New("not found")
var ErrEmptyDll = errors.New("empty dll")

// ErrInternalDll indicates an internal inconsistency in the DLL.
var ErrInternalDll = errors.New("internal dll error")

// ErrOutOfRange indicates that a subscript is out of range.
var ErrOutOfRange = errors.New("subscript out of range")

// Pop will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) Pop() (rv *T, err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	rv, err = ns.noLockPop()
	return
}

// PopTail will remove the bottom element from the DLL.  An error is returned if the list is empty.
func (ns *Dll[T]) PopTail() (rv *T, err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	rv, err = ns.noLockPopTail()
	return
}

// noLockPop will remove the top element from the DLL.  The caller must hold the lock.
func (ns *Dll[T]) noLockPop() (rv *T, err error) {
	if ns.length == 0 {
		return nil, ErrEmptyDll
	}
	rm := ns.head
	rv = rm.Data
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

// noLockPopTail will remove the bottom element from the DLL.  The caller must hold the lock.
func (ns *Dll[T]) noLockPopTail() (rv *T, err error) {
	if ns.length == 0 {
		return nil, ErrEmptyDll
	}
	rm := ns.tail
	rv = rm.Data
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

// Delete removes a matching element, searching from head to tail.
// If the element is not found an error is returned.
func (ns *Dll[T]) Delete(t *T) (err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	for p := ns.head; p != nil; p = p.next {
		if (*p.Data).IsEqual(*t) { // IsEqual(b Equality) bool
			return ns.noLockDelete(p)
		}
	}
	return ErrNotFound
}

// DeleteSearch will search for a node matching the supplied 't' and if a match is found then that
// node will be deleted.  The search is a linear search from the head.  If it is not found then
// an error is returned.  This is an alias for Delete.
func (ns *Dll[T]) DeleteSearch(t *T) (err error) {
	return ns.Delete(t)
}

// DeleteFound removes a 'found' element from the DLL, the next/prev
// pointers must be in this list.
func (ns *Dll[T]) DeleteFound(it *DllElement[T]) (err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return ns.noLockDelete(it)
}

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

// Peek returns the top element of the DLL (like a Stack) or an error indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv *T, err error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, ErrEmptyDll
	}
	rv = ns.head.Data
	return
}

// PeekTail returns the last element of the DLL (like a Queue) or an error indicating that the queue is empty.
func (ns *Dll[T]) PeekTail() (rv *T, err error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, ErrEmptyDll
	}
	rv = ns.tail.Data
	return
}

// Truncate removes all data from the list.
func (ns *Dll[T]) Truncate() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
// If the item is not found then a position of -1 is returned.
func (ns *Dll[T]) Search(t *T) (rv *DllElement[T], pos int) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := 0
	for p := ns.head; p != nil; p = p.next {
		if (*p.Data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
// If the item is not found then a position of -1 is returned.
func (ns *Dll[T]) ReverseSearch(t *T) (rv *DllElement[T], pos int) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := ns.length - 1
	for p := ns.tail; p != nil; p = p.prev {
		if (*p.Data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

// ApplyFunction is the type of the callback used by Walk and ReverseWalk.
// It returns true to stop the iteration (the current element is returned by Walk).
type ApplyFunction[T comparable.Equality] func(pos int, data T, userData interface{}) bool

// Walk - Iterate from head to tail of list. 												O(n)
// If fx returns true the walk stops and the current element and its position are returned.
// If the walk completes without fx returning true then nil, -1 is returned.
//
// A read lock is held while fx runs, so fx must not call back into the list.
func (ns *Dll[T]) Walk(fx ApplyFunction[T], userData interface{}) (rv *DllElement[T], pos int) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := 0
	for p := ns.head; p != nil; p = p.next {
		if fx(i, *p.Data, userData) {
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseWalk - Iterate from tail to head of list. 											O(n)
// If fx returns true the walk stops and the current element and its position are returned.
// If the walk completes without fx returning true then nil, -1 is returned.
//
// A read lock is held while fx runs, so fx must not call back into the list.
func (ns *Dll[T]) ReverseWalk(fx ApplyFunction[T], userData interface{}) (rv *DllElement[T], pos int) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.length == 0 {
		return nil, -1 // not found
	}

	i := ns.length - 1
	for p := ns.tail; p != nil; p = p.prev {
		if fx(i, *p.Data, userData) {
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

// Index will return the Nth item from the list.
func (ns *Dll[T]) Index(sub int) (rv *DllElement[T], err error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
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
func (ns *Dll[T]) IndexFromTail(sub int) (rv *DllElement[T], err error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
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
// Complexity is O(n).
func (ns *Dll[T]) Trim(n int) (err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

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
// Complexity is O(n).
func (ns *Dll[T]) TrimTail(n int) (err error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

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
// Complexity is O(len(yy)).
func (ns *Dll[T]) Concat(yy *Dll[T]) {
	// Snapshot the data of yy under its read lock.  The two locks are never
	// held at the same time, so Concat is safe even when ns == yy and there
	// is no lock-ordering hazard.
	yy.mu.RLock()
	data := make([]*T, 0, yy.length)
	for ptr := yy.head; ptr != nil; ptr = ptr.next {
		data = append(data, ptr.Data)
	}
	yy.mu.RUnlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()
	for _, d := range data {
		ns.noLockAppendAtTail(d)
	}
}

// Reverse - efficiently reverse direction on a list.  O(n) with storage O(1)
func (ns *Dll[T]) Reverse() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	var next *DllElement[T]

	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next, cp.prev = cp.prev, cp.next
	}

	ns.head, ns.tail = ns.tail, ns.head
}

// Dump prints the list, one element per line, to fo.
func (ns *Dll[T]) Dump(fo io.Writer) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	i := 0
	for p := ns.head; p != nil; p = p.next {
		_, _ = fmt.Fprintf(fo, "%d: %+v\n", i, *(p.Data))
		i++
	}
}

// Lock takes the list's write lock.  It must be paired with a call to Unlock.
// No other list operation may be performed by any goroutine while the lock is
// held, so prefer the built-in locked operations where possible.
func (ns *Dll[T]) Lock() {
	ns.mu.Lock()
}

// Unlock releases the list's write lock taken by Lock.
func (ns *Dll[T]) Unlock() {
	ns.mu.Unlock()
}

// -----------------------------------------------------------------------------------------------------------
// Go 1.23+ range-over-func iterators.
//
// The iterators operate on a consistent snapshot of the list taken under a
// read lock, so the lock is not held while user code in the loop body runs
// and the loop body may safely call other list methods.

// snapshot returns the data pointers of the list under a read lock.
func (ns *Dll[T]) snapshot() []*T {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	data := make([]*T, 0, ns.length)
	for p := ns.head; p != nil; p = p.next {
		data = append(data, p.Data)
	}
	return data
}

// All returns an iterator over the elements of the list from head to tail.
// The index starts at 0 at the head of the list.
func (ns *Dll[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, d := range ns.snapshot() {
			if !yield(i, *d) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the list from tail to head.
// The index starts at Length()-1 at the tail of the list.
func (ns *Dll[T]) Backward() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		data := ns.snapshot()
		for i := len(data) - 1; i >= 0; i-- {
			if !yield(i, *data[i]) {
				return
			}
		}
	}
}

// IterateOver is a legacy name for All.
func (ns *Dll[T]) IterateOver() iter.Seq2[int, T] {
	return ns.All()
}

// IteratePtr returns an iterator over pointers to the elements of the list
// from head to tail.
func (ns *Dll[T]) IteratePtr() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		for i, d := range ns.snapshot() {
			if !yield(i, d) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
