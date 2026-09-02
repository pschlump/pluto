/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package dll implements a generic doubly linked list (DLL) with head-and-tail
// pointers.
//
// Elements are stored and returned by value, so element
// data is never boxed into an interface and never unboxed with a type
// assertion.  Lists of types that can be compared with == (the builtin
// comparable constraint — all scalars, strings, arrays, and structs of
// comparable fields) are created with NewDll, which compares elements with
// the == operator; lists of any other type — or with field-based equality
// — are created with NewDllFunc, which takes a caller supplied equality
// function; the element type does not have to implement any interface.
// The equality function is only consulted by Search, ReverseSearch and the
// Delete calls built on them; the stack/queue and positional operations
// never order or compare elements.
//
// Operations:
//
//	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
//	Delete — Deletes a specified element from the linked list (element can be found via Search). O(n)
//	DeleteFound — Deletes a previously found element (from Search/ReverseSearch/Index).		O(1)
//	DeleteSearch — Deletes a specified element, searching from head to tail.					O(n)
//	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
//	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
//	Index — Return the Nth item in the list - in a format usable with DeleteFound.				O(n) n/2
//	IndexFromTail — Return the Nth item from the tail of the list.								O(n) n/2
//	InsertBeforeHead — Inserts a new element before the current first element of list.  		O(1)
//	IsEmpty — Returns true if the linked list is empty											O(1)
//	Len / Length — Returns number of elements in the list.  0 length is an empty list.			O(1)
//	Peek — Look at data at head of list.														O(1)
//	PeekTail — Look at data at tail of list.													O(1)
//	Pop	— Remove and return from the head of the list.											O(1)
//	PopTail — Remove and return from the tail of the list.										O(1)
//	Push — Insert at the head of the list.														O(1)
//	ReverseList — Reverse all the nodes in list. 												O(n)
//	Reverse — Reverse all the nodes in list with O(1) extra storage.							O(n)
//	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
//	ReverseWalk — Iterate from tail to head of list. 											O(n)
//	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n) n/2
//	Truncate — Delete all the nodes in list. 													O(1)
//	Walk — Iterate from head to tail of list. 													O(n)
//	Trim — Cut list to specified length, keeping the head; unchanged if shorter.				O(n)
//	TrimTail — Cut list to specified length, keeping the tail; unchanged if shorter.			O(n)
//	Concat — Append a copy of the elements of another list to this list.						O(n)
//
// The list also implements the json.Marshaler and json.Unmarshaler
// interfaces (MarshalJSON/UnmarshalJSON, in json.go): it encodes as a
// JSON array of its elements, head to tail, and unmarshaling replaces
// the contents.
//
// With the basic stack operations it also can be used as a stack:
//
//	Push — Inserts an element at the top														O(1)
//	Pop — Will remove the top element from the stack.  ErrEmptyDll is returned if the stack	O(1)
//		is empty.
//	IsEmpty — Returns true if the stack is empty												O(1)
//	Peek — Returns the top element without removing from the stack								O(1)
//
// With the use of Enqueue it can be used as a queue.  Enqueue is a synonym for AppendAtTail.	O(1)
//
//	PeekTail — Peek returns the last element of the DLL (like a queue) or ErrEmptyDll.			O(1)
//	PopTail — Remove the element at the end of the DLL.											O(1)
//	Enqueue — Add to the tail so that DLL can be used as a queue.								O(1)
//
// Iteration is supported by the legacy DllIter type (Front/Rear/Current with
// Next/Prev/Done/Pos/Value), by the Walk/ReverseWalk callbacks, and by Go
// 1.23+ range-over-func iterators:
//
//	All — iter.Seq2[int, T] from head to tail.													O(n)
//	Backward — iter.Seq2[int, T] from tail to head.												O(n)
//	IterateOver — legacy name for All.															O(n)
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
// The insert family is InsertBeforeHead, Push, AppendAtTail, Enqueue,
// Concat (which appends) and UnmarshalJSON (when the data holds
// elements).
//
// This version of the DLL is not suitable for concurrent usage; a mutex
// guarded thread-safe twin has the exact same interface.
package dll

import (
	"errors"
	"fmt"
	"io"
	"iter"
)

// DllElement is an element in the doubly linked list.
type DllElement[T any] struct {
	data       T
	next, prev *DllElement[T]
}

// Dll is a generic doubly linked list with head and tail pointers.
// Use NewDll for element types that support ==, or NewDllFunc for a
// caller supplied equality function.  The zero value is an empty list.
type Dll[T any] struct {
	head, tail *DllElement[T]
	length     int

	// eq reports whether two elements are considered the same.  It is set
	// by the constructors and is the only thing that knows how to compare
	// T — T itself never has to implement an interface.  It is consulted
	// only by Search, ReverseSearch and the Delete calls built on them.
	eq func(a, b T) bool
}

// DllIter is an iteration type that allows a for loop to walk the list in
// either direction.
type DllIter[T any] struct {
	cur *DllElement[T]
	pos int
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
		panic("dll: NewDllFunc called with a nil equality function")
	}
	return &Dll[T]{eq: fx}
}

// equal compares a and b, guarding against a zero-value list that was not
// created by one of the constructors.
func (ns *Dll[T]) equal(a, b T) bool {
	if ns.eq == nil {
		panic("dll: no equality function (create the list with NewDll or NewDllFunc)")
	}
	return ns.eq(a, b)
}

// GetData returns the data stored in the element.
// Complexity is O(1).
func (ee *DllElement[T]) GetData() T {
	return ee.data
}

// SetData sets the data stored in the element.
// Complexity is O(1).
func (ee *DllElement[T]) SetData(d T) {
	ee.data = d
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the beginning of a list for iteration over list.
func (ns *Dll[T]) Front() *DllIter[T] {
	if ns == nil {
		return &DllIter[T]{}
	}
	return &DllIter[T]{cur: ns.head}
}

// Rear will start at the end of a list for iteration over list.
func (ns *Dll[T]) Rear() *DllIter[T] {
	if ns == nil {
		return &DllIter[T]{pos: -1}
	}
	return &DllIter[T]{cur: ns.tail, pos: ns.length - 1}
}

// Current will take the node returned from Search or ReverseSearch
//
//	func (ns *Dll[T]) Search( t T ) (rv *DllElement[T], pos int)
//
// and allow you to start an iteration process from that point.
func (ns *Dll[T]) Current(el *DllElement[T], pos int) *DllIter[T] {
	return &DllIter[T]{cur: el, pos: pos}
}

// Value returns the current data for this element in the list, or false
// if the iteration is done.
func (iter *DllIter[T]) Value() (rv T, found bool) {
	if iter.cur != nil {
		return iter.cur.data, true
	}
	return
}

// Next advances to the next element in the list.
func (iter *DllIter[T]) Next() {
	if iter.cur == nil {
		return
	}
	iter.cur = iter.cur.next
	iter.pos++
}

// Prev moves back to the previous element in the list.
func (iter *DllIter[T]) Prev() {
	if iter.cur == nil {
		return
	}
	iter.cur = iter.cur.prev
	iter.pos--
}

// Done returns true if the end of the list has been reached.
func (iter *DllIter[T]) Done() bool {
	return iter.cur == nil
}

// Pos returns the current "index" of the element being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *DllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the DLL (queue or stack) is empty.
func (ns *Dll[T]) IsEmpty() bool {
	return ns == nil || ns.length == 0
}

// InsertBeforeHead will insert a new node at the head of the list.
// It returns true if the node was inserted.
// It panics on a nil list or on a zero-value list (no equality
// function); see the package documentation for the panic contract.
func (ns *Dll[T]) InsertBeforeHead(t T) bool {
	if ns == nil {
		panic("dll: InsertBeforeHead called on a nil list")
	}
	if ns.eq == nil {
		panic("dll: InsertBeforeHead called on a list with no equality function (create the list with NewDll or NewDllFunc)")
	}
	x := &DllElement[T]{data: t} // Create the node
	if ns.head == nil {
		ns.head = x
		ns.tail = x
		ns.length = 1
		return true
	}
	x.next = ns.head
	ns.head.prev = x
	ns.head = x
	ns.length++
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
		panic("dll: AppendAtTail called on a nil list")
	}
	if ns.eq == nil {
		panic("dll: AppendAtTail called on a list with no equality function (create the list with NewDll or NewDllFunc)")
	}
	x := &DllElement[T]{data: t} // Create the node
	if ns.tail == nil {
		ns.head = x
		ns.tail = x
		ns.length = 1
		return true
	}
	x.prev = ns.tail
	ns.tail.next = x
	ns.tail = x
	ns.length++
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
	return ns.length
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	if ns == nil {
		return 0
	}
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
	if ns.IsEmpty() {
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

// PopTail will remove the bottom element from the DLL.  ErrEmptyDll is
// returned if the list is empty.
func (ns *Dll[T]) PopTail() (rv T, err error) {
	if ns.IsEmpty() {
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

// Delete removes a matching element, searching from head to tail.
// If the element is not found ErrNotFound is returned.
func (ns *Dll[T]) Delete(t T) (err error) {
	it, pos := ns.Search(t)
	if pos < 0 {
		return ErrNotFound
	}
	return ns.DeleteFound(it)
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
		_, err = ns.Pop()
		return
	}
	if ns.tail == it && ns.length > 1 {
		_, err = ns.PopTail()
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

// Peek returns the top element of the DLL (like a Stack) or ErrEmptyDll
// indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv T, err error) {
	if ns.IsEmpty() {
		return rv, ErrEmptyDll
	}
	return ns.head.data, nil
}

// PeekTail returns the last element of the DLL (like a Queue) or
// ErrEmptyDll indicating that the queue is empty.
func (ns *Dll[T]) PeekTail() (rv T, err error) {
	if ns.IsEmpty() {
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
	ns.head = nil
	ns.tail = nil
	ns.length = 0
}

// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
// If the item is not found then a position of -1 is returned.
// The probe only needs the fields that the equality function reads.
func (ns *Dll[T]) Search(t T) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
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
	if ns.IsEmpty() {
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
func (ns *Dll[T]) Walk(fx ApplyFunction[T]) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
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
func (ns *Dll[T]) ReverseWalk(fx ApplyFunction[T]) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
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
	if ns.IsEmpty() {
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
	if ns.IsEmpty() {
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
	if ns.IsEmpty() {
		return ErrEmptyDll
	}
	if n <= 0 {
		ns.Truncate()
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
	if ns.IsEmpty() {
		return ErrEmptyDll
	}
	if n <= 0 {
		ns.Truncate()
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
// Concat follows the insert contract: ns must have been created with
// NewDll or NewDllFunc.
// Complexity is O(len(yy)).
func (ns *Dll[T]) Concat(yy *Dll[T]) {
	if yy == nil {
		return
	}
	n := yy.length
	for ptr := yy.head; ptr != nil && n > 0; ptr = ptr.next {
		ns.AppendAtTail(ptr.data)
		n--
	}
}

// Dump prints the list, one element per line, to fo.
func (ns *Dll[T]) Dump(fo io.Writer) {
	if ns == nil {
		return
	}
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
	var next *DllElement[T]

	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next, cp.prev = cp.prev, cp.next
	}

	ns.head, ns.tail = ns.tail, ns.head
}

// Lock is a no-op provided for API compatibility with a thread-safe dll
// twin.  This implementation is not safe for concurrent use.
func (ns *Dll[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with a thread-safe dll
// twin.  This implementation is not safe for concurrent use.
func (ns *Dll[T]) Unlock() {}

// -----------------------------------------------------------------------------------------------------------
// Go 1.23+ range-over-func iterators.

// All returns an iterator over the elements of the list from head to tail.
// The index starts at 0 at the head of the list.
// The list must not be modified while the iterator is being consumed — it
// walks the live nodes.
func (ns *Dll[T]) All() iter.Seq2[int, T] {
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

// Backward returns an iterator over the elements of the list from tail to head.
// The index starts at Length()-1 at the tail of the list.
// The list must not be modified while the iterator is being consumed — it
// walks the live nodes.
func (ns *Dll[T]) Backward() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i, p := ns.length-1, ns.tail; p != nil; i, p = i-1, p.prev {
			if !yield(i, p.data) {
				return
			}
		}
	}
}

// IterateOver is a legacy name for All.
func (ns *Dll[T]) IterateOver() iter.Seq2[int, T] {
	return ns.All()
}
