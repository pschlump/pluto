package dll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Basic operations on a Doubly Linked List (DLL).
This list has head-and-tail pointers.

*	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
*	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)
*	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
*	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
*	Index - return the Nth item	in the list - in a format usable with Delete.					O(n) n/2
*	InsertBeforeHead — Inserts a new element before the current first ement of list.  			O(1)
*	IsEmpty — Returns true if the linked list is empty											O(1)
*	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
*	Peek - Look at data at head of list.														O(1)
*	Pop	- Remove and return from the head of the list.											O(1)
*	Push - Insert at the head of the list.														O(1)
*	ReverseList - Reverse all the nodes in list. 												O(n)
*	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
*	ReverseWalk - Iterate from tail to head of list. 											O(n)
*	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n) n/2
*	Truncate - Delete all the nodes in list. 													O(1)
*	Walk - Iterate from head to tail of list. 													O(n)

With the basic stack operations it also can be used as a stack:
*	Push — Inserts an element at the top														O(1)
*	Pop - will remove the top element from the stack.  An error is returned if the stack is		O(1)
		empty.
*	IsEmpty — Returns true if the stack is empty												O(1)
*	Peek — Returns the top element without removing from the stack								O(1)

With the use of Enque can be used as a Queue.  This is a synonym for AppendAtTail.				O(1)

* 	PeekTail - Peek returns the last element of the DLL (like a Queue) or an error 				O(1)
		indicating that the queue is empty.
* 	PopTail - Remvoe the element at the end of the DLL.											O(1)
*	Enque - add to the tail so that DLL can be used as a Queue.									O(1)

Additional go1.22 Functionality (replacements for Walk, ReverseWalk)
All of this code can be found at the very bottom of this file. (except the type DllDeq)
This replaces the DllIter type and Front/Done/Next/Value.

*	DllSeq					The Type for the Iterator Sequence

This version of the DLL is not suitable for concurrnet usage but ../DLLTs has mutex
locks so that it is thread safe.  It has the exact same interface.

*/

import (
	"errors"
	"fmt"
	"io"

	"github.com/pschlump/pluto/comparable"
)

// To iterate over a list (where e is a *dll.Dll):
//	for e := l.Front(); e.Done(); e.Next() {
//		// do something with e.Value()
//	}
// or
//	for e := l.Rear(); e.Done(); e.Prev() {
//		// do something with e.Value()
//	}
//

// An element in the doubly linked list.
type DllElement[T comparable.Equality] struct {
	next, prev *DllElement[T]
	Data       *T
}

// Dll is a generic type buildt on top of a slice
type Dll[T comparable.Equality] struct {
	head, tail *DllElement[T]
	length     int
}

// An iteration type that allows a for loop to walk the list.
type DllIter[T comparable.Equality] struct {
	cur *DllElement[T]
	dll *Dll[T]
	pos int
}

type DllSeq[V comparable.Equality] func(yield func(V) bool)

// -------------------------------------------------------------------------------------------------------

// Create a new DLL and return it.
// Complexity is O(1).
func NewDll[T comparable.Equality]() *Dll[T] {
	return &Dll[T]{
		head:   nil,
		tail:   nil,
		length: 0,
	}
}

// Complexity is O(1).
func (ee *DllElement[T]) GetData() *T {
	return ee.Data
}

// Complexity is O(1).
func (ee *DllElement[T]) SetData(d *T) {
	ee.Data = d
}

// -------------------------------------------------------------------------------------------------------

// Front will start at the beginning of a list for iteration over list.
func (ns *Dll[T]) Front() *DllIter[T] {
	return &DllIter[T]{
		cur: ns.head,
		dll: ns,
	}
}

// Rear will start at the end of a list for iteration over list.
func (ns *Dll[T]) Rear() *DllIter[T] {
	return &DllIter[T]{
		cur: ns.tail,
		dll: ns,
		pos: ns.length - 1,
	}
}

// Current will take the node returned from Search or RevrseSearch
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
	if iter.cur != nil {
		return iter.cur.Data
	}
	return nil
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

// Pos returns the current "index" of the elemnt being iterated on.  So if the list has 3 elements, a, b, c and we
// start at the head of the list 'a' will have a Pos() of 0, 'b' will have a Pos() of 1 etc.
func (iter *DllIter[T]) Pos() int {
	return iter.pos
}

// -------------------------------------------------------------------------------------------------------

// IsEmpty will return true if the DLL (queue or stack) is empty
func (ns *Dll[T]) IsEmpty() bool {
	return (*ns).length == 0
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) InsertBeforeHead(t *T) bool {
	x := DllElement[T]{Data: t} // Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
		return true
	} else {
		// xyzzy - TODO - if duplicate then replace?
		x.next = (*ns).head
		(*ns).head.prev = &x
		(*ns).head = &x
		(*ns).length++
		return true
	}
}
func (ns *Dll[T]) Push(t *T) {
	ns.InsertBeforeHead(t)
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) AppendAtTail(t *T) bool {
	x := DllElement[T]{Data: t} // Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
		return true
	} else {
		// xyzzy - TODO - if duplicate then replace?
		x.prev = (*ns).tail
		(*ns).tail.next = &x
		(*ns).tail = &x
		(*ns).length++
		return true
	}
}

func (ns *Dll[T]) Enque(t *T) {
	(*ns).AppendAtTail(t)
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	return (*ns).length
}

// An error to indicate that the DLL is empty
var ErrEmptyDll = errors.New("Empty Dll")
var ErrInteralDll = errors.New("Interal Dll")
var ErrOutOfRange = errors.New("Subscript Out of Range")

// Pop will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) Pop() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).head.Data
	(*ns).head = (*ns).head.next
	if (*ns).head != nil {
		(*ns).head.prev = nil
	}
	(*ns).length--
	return
}

// PopTail will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) PopTail() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).tail.Data
	(*ns).tail = (*ns).tail.prev
	if (*ns).tail != nil {
		(*ns).tail.next = nil
	}
	(*ns).length--
	return
}

// Delete a matching element
func (ns *Dll[T]) Delete(t *T) (err error) {
	it, pos := ns.Search(t)
	if pos < 0 {
		return fmt.Errorf("Not Found in DLL")
	}
	return ns.DeleteFound(it)
}

// Delete removes a 'found' element from the DLL, the next/prev
// pointers must be in this list.
func (ns *Dll[T]) DeleteFound(it *DllElement[T]) (err error) {
	if (*ns).head == it && (*ns).tail == it {
		(*ns).head = nil
		(*ns).tail = nil
		(*ns).length = 0
		return
	}
	if (*ns).head == it && (*ns).length > 1 {
		err = ns.DeleteAtHead()
		return
	}
	if (*ns).tail == it && (*ns).length > 1 {
		err = ns.DeleteAtTail()
		return
	}
	if (*ns).length > 2 {
		n := it.prev
		p := it.next
		n.next = p
		p.prev = n
		(*ns).length--
		return
	}
	return ErrInteralDll
}

func (ns *Dll[T]) DeleteAtHead() (err error) {
	_, err = ns.Pop()
	return
}

func (ns *Dll[T]) DeleteAtTail() (err error) {
	if ns.IsEmpty() {
		return ErrEmptyDll
	}
	// rv = (*ns).tail.Data
	(*ns).tail = (*ns).tail.prev
	if (*ns).tail != nil {
		(*ns).tail.next = nil
	}
	(*ns).length--
	return
}

// Peek returns the top element of the DLL (like a Stack) or an error indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).head.Data
	return
}

// Peek returns the last element of the DLL (like a Queue) or an error indicating that the stack is empty.
func (ns *Dll[T]) PeekTail() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).tail.Data
	return
}

// Truncate removes all data from the list.
func (ns *Dll[T]) Truncate() {
	(*ns).head = nil
	(*ns).tail = nil
	(*ns).length = 0
	return
}

// Walk - Iterate from head to tail of list. 												O(n)
// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
// If the item is not found then a position of -1 is returned.
func (ns *Dll[T]) Search(t *T) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		if (*p.Data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
func (ns *Dll[T]) ReverseSearch(t *T) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	i := (*ns).length
	for p := (*ns).tail; p != nil; p = p.prev {
		if (*p.Data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

type ApplyFunction[T comparable.Equality] func(pos int, data T, userData interface{}) bool

// Walk - Iterate from head to tail of list. 												O(n)
func (ns *Dll[T]) Walk(fx ApplyFunction[T], userData interface{}) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		if fx(i, *p.Data, userData) {
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseWalk - Iterate from tail to head of list. 											O(n)
func (ns *Dll[T]) ReverseWalk(fx ApplyFunction[T], userData interface{}) (rv *DllElement[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	}

	i := (*ns).length
	for p := (*ns).tail; p != nil; p = p.prev {
		if fx(i, *p.Data, userData) {
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

// ReverseList - Reverse all the nodes in list. 												O(n)
func (ns *Dll[T]) ReverseList() {
	if ns.IsEmpty() {
		return
	}

	var tmp Dll[T]
	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		tmp.InsertBeforeHead(p.Data)
		i++
	}
	ns.head = tmp.head
	ns.tail = tmp.tail
}

// Index will return the Nth item from the list.
func (ns *Dll[T]) Index(sub int) (rv *DllElement[T], err error) {
	if ns.IsEmpty() {
		return nil, ErrOutOfRange
	}

	if sub < 0 || sub >= (*ns).length {
		return nil, ErrOutOfRange
	} else if sub < ((*ns).length / 2) {
		i := 0
		rv = (*ns).head
		for ; i < sub; rv = rv.next {
			i++
		}
		return
	} else {
		i := (*ns).length - 1
		rv = (*ns).tail
		for ; rv != nil && i > sub; rv = rv.prev {
			i--
		}
		return
	}

	// return nil, ErrOutOfRange
}

func (tt *Dll[T]) Dump(fo io.Writer) {
	i := 0
	for p := tt.head; p != nil; p = p.next {
		fmt.Fprintf(fo, "%d: %+v\n", i, *(p.Data))
		i++
	}
}

// Reverse - effeciently reverse direciotn on a list.  O(n) with storage O(1)
func (ns *Dll[T]) Reverse() {

	var next *DllElement[T]

	for cp := ns.head; cp != nil; cp = next {
		next = cp.next // save next pointer at beginning
		cp.next, cp.prev = cp.prev, cp.next
	}

	ns.head, ns.tail = ns.tail, ns.head

}

// -----------------------------------------------------------------------------------------------------------
// Go1.22 Iterator stuff

// Type declared above
// type DllSeq[V comparable.Equality] func(yield func(V) bool)

/*
func All[T any]( yield func(ns *Dll[T]) ( Dll[T] , bool ) {
	ii := 0
	for nn := ns.Head; nn != nil; nn = nn->Next {
		if !yield(ii, nn) {
			return
		}
		ii++
	}
}

func ( ns * Dll[T]) All[E any](s []E) func(func(int, E) bool) {
	return func(yield func(int, E) bool) {
		for i := len(s) - 1; i >= 0; i-- {
			if !yield(i, s[i]) {
				// Where clean-up code goes
				return
			}
		}
	}
}
*/

/* vim: set noai ts=4 sw=4: */
