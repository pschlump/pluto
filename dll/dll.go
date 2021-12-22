package dll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Basic operations on a Doubly Linked List (DLL).
This list has head-and-tail pointers.

*	AppendAtTail — Inserts a new element after the end of the linked list.  					O(1)
*	DeleteAtHead — Deletes the first element of the linked list.  								O(1)
*	DeleteAtTail — Deletes the last element of the linked list. 								O(1)
*	InsertBeforeHead — Inserts a new element before the current first ement of list.  			O(1)
*	IsEmpty — Returns true if the linked list is empty											O(1)
*	Length — Returns number of elements in the list.  0 length is an empty list.				O(1)
*	Peek																						O(1)
*	Pop																							O(1)
*	Push																						O(1)
*	ReverseList - Reverse all the nodes in list. 												O(n)
*	Truncate - Delete all the nodes in list. 													O(1)
*	Walk - Iterate from head to tail of list. 													O(n)
*	ReverseWalk - Iterate from tail to head of list. 											O(n)

+	Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
+	ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
+	Delete — Deletes a specified element from the linked list (Element can be fond via Search). O(1)

*/

import (
	"errors"

	"github.com/pschlump/pluto/comparable"
)

// A node in the singly linked list
type DllNode[T comparable.Equality] struct {
	next, prev *DllNode[T]
	data *T
}
// Dll is a generic type buildt on top of a slice
type Dll[T comparable.Equality] struct {
	head, tail *DllNode[T]
	length int
}

// IsEmpty will return true if the stack is empty
func (ns *Dll[T]) IsEmpty() bool {
	return (*ns).length == 0
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) InsertBeforeHead(t *T) {
	x := DllNode[T] { data: t }	// Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
	} else {
		x.next = (*ns).head
		(*ns).head.prev = &x
		(*ns).head = &x
		(*ns).length++
	}
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) AppendAtTail(t *T) {
	x := DllNode[T] { data: t }	// Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
	} else {
		x.prev = (*ns).tail
		(*ns).tail.next = &x
		(*ns).tail = &x
		(*ns).length++
	}
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	return (*ns).length
}

// An error to indicate that the stack is empty
var ErrEmptyDll = errors.New("Empty Dll")
var ErrInteralDll = errors.New("Interal Dll")

// Pop will remove the top element from the stack.  An error is returned if the stack is empty.
func (ns *Dll[T]) Pop() ( rv *T, err error ) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).head.data
	(*ns).head = (*ns).head.next
	if (*ns).head != nil {
		(*ns).head.prev = nil
	}
	(*ns).length--
	return 
}

func (ns *Dll[T]) Delete( it *DllNode[T] ) ( err error ) {
	_, err = ns.Pop()
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
		// xyzzy - TODO - 
		// xyzzy - TODO - 
		// xyzzy - TODO - 
		// xyzzy - TODO - 
		n := it.prev
		p := it.next
		n.next = p
		p.prev = n
		return
	}
	return ErrInteralDll 
}

func (ns *Dll[T]) DeleteAtHead() ( err error ) {
	_, err =ns.Pop()
	return
}

func (ns *Dll[T]) DeleteAtTail() ( err error ) {
	if ns.IsEmpty() {
		return ErrEmptyDll
	}
	// rv = (*ns).tail.data
	(*ns).tail = (*ns).tail.prev
	if (*ns).tail != nil {
		(*ns).tail.next = nil
	}
	(*ns).length--
	return 
}

// Peek returns the top element of the stack or an error indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	} 
	rv = (*ns).head.data
	return 
}

// Truncate removes all data from the list.
func (ns *Dll[T]) Truncate()  {
	(*ns).head = nil
   	(*ns).tail = nil
	(*ns).length = 0
	return 
}

// Walk - Iterate from head to tail of list. 												O(n)
// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
func (ns *Dll[T]) Search( t *T ) (rv *DllNode[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	} 

	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		if (*p.data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseSearch — Returns the given element from a linked list searching from tail to head.	O(n)
func (ns *Dll[T]) ReverseSearch( t *T ) (rv *DllNode[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	} 

	i := (*ns).length
	for p := (*ns).tail; p != nil; p = p.prev {
		if (*p.data).IsEqual(*t) { // IsEqual(b Equality) bool
			return p, i
		}
		i--
	}
	return nil, -1 // not found
}

type ApplyFunction[T comparable.Equality] func ( pos int, data T, userData interface{} ) bool

// Walk - Iterate from head to tail of list. 												O(n)
func (ns *Dll[T]) Walk( fx ApplyFunction[T], userData interface{} ) (rv *DllNode[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	} 

	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		if fx(i, *p.data, userData) { 
			return p, i
		}
		i++
	}
	return nil, -1 // not found
}

// ReverseWalk - Iterate from tail to head of list. 											O(n)
func (ns *Dll[T]) ReverseWalk( fx ApplyFunction[T], userData interface{} ) (rv *DllNode[T], pos int) {
	if ns.IsEmpty() {
		return nil, -1 // not found
	} 

	i := (*ns).length
	for p := (*ns).tail; p != nil; p = p.prev {
		if fx(i, *p.data, userData) { 
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
		// tmp.AppendAtTail(p.data)
		tmp.InsertBeforeHead(p.data)
		i++
	}
	ns.head = tmp.head
	ns.tail = tmp.tail
}

