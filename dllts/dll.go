package dllts

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

This version of the DLL is not suitable for concurrnet usage but ../DLLTs has mutex 
locks so that it is thread safe.  It has the exact same interface.

*/

import (
	"errors"
	"sync"

	"github.com/pschlump/pluto/comparable"
)

// A node in the singly linked list
type DllNode[T comparable.Equality] struct {
	next, prev 	*DllNode[T]
	data 		*T
}
// Dll is a generic type buildt on top of a slice
type Dll[T comparable.Equality] struct {
	head, tail 	*DllNode[T]
	length 		int
	mu       	sync.RWMutex
}

// IsEmpty will return true if the DLL (queue or stack) is empty
func (ns *Dll[T]) IsEmpty() bool {
	return (*ns).length == 0
}

func (ns *Dll[T]) noLockInsertBeforeHead(t *T) {
	x := DllNode[T] { data: t }	// Create the node
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
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
func (ns *Dll[T]) InsertBeforeHead(t *T) {
	(*ns).mu.Lock()
	(*ns).noLockInsertBeforeHead(t)
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) AppendAtTail(t *T) {
	x := DllNode[T] { data: t }	// Create the node
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
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

func (ns *Dll[T]) Enque(t *T) {
	(*ns).AppendAtTail(t)
}

// Length returns the number of elements in the list.
func (ns *Dll[T]) Length() int {
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
	return (*ns).length
}

// An error to indicate that the DLL is empty
var ErrEmptyDll = errors.New("Empty Dll")
var ErrInteralDll = errors.New("Interal Dll")
var ErrOutOfRange = errors.New("Subscript Out of Range")

// Pop will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) Pop() ( rv *T, err error ) {
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	rv, err = (*ns).noLockPop()
	return
}

// PopTail will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) PopTail() ( rv *T, err error ) {
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	rv, err = (*ns).noLockPopTail()
	return
}

// Pop will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) noLockPop() ( rv *T, err error ) {
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

// PopTail will remove the top element from the DLL.  An error is returned if the stack is empty.
func (ns *Dll[T]) noLockPopTail() ( rv *T, err error ) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	}
	rv = (*ns).tail.data
	(*ns).tail = (*ns).tail.prev
	if (*ns).tail != nil {
		(*ns).tail.next = nil
	}
	(*ns).length--
	return 
}

func (ns *Dll[T]) Delete( it *DllNode[T] ) ( err error ) {
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	if (*ns).head == it && (*ns).tail == it {
		(*ns).head = nil
		(*ns).tail = nil
		(*ns).length = 0
		return
	}
	if (*ns).head == it && (*ns).length > 1 {
		_, err = (*ns).noLockPop() 
		return
	}
	if (*ns).tail == it && (*ns).length > 1 {
		_, err = (*ns).noLockPopTail() 
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

func (ns *Dll[T]) DeleteAtHead() ( err error ) {
	_, err = ns.Pop()
	return
}

func (ns *Dll[T]) DeleteAtTail() ( err error ) {
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	if ns.IsEmpty() {
		return ErrEmptyDll
	}
	(*ns).tail = (*ns).tail.prev
	if (*ns).tail != nil {
		(*ns).tail.next = nil
	}
	(*ns).length--
	return 
}

// Peek returns the top element of the DLL (like a Stack) or an error indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv *T, err error) {
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	} 
	rv = (*ns).head.data
	return 
}

// Peek returns the last element of the DLL (like a Queue) or an error indicating that the stack is empty.
func (ns *Dll[T]) PeekTail() (rv *T, err error) {
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	} 
	rv = (*ns).tail.data
	return 
}

// Truncate removes all data from the list.
func (ns *Dll[T]) Truncate()  {
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	(*ns).head = nil
   	(*ns).tail = nil
	(*ns).length = 0
	return 
}

// Walk - Iterate from head to tail of list. 												O(n)
// Search — Returns the given element from a linked list.  Search is from head to tail.		O(n)
func (ns *Dll[T]) Search( t *T ) (rv *DllNode[T], pos int) {
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
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
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
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
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
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
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
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
	(*ns).mu.Lock()
	defer (*ns).mu.Unlock()
	if ns.IsEmpty() {
		return 
	} 

	var tmp Dll[T]
	i := 0
	for p := (*ns).head; p != nil; p = p.next {
		tmp.noLockInsertBeforeHead(p.data)
		i++
	}
	ns.head = tmp.head
	ns.tail = tmp.tail
}

// Index will return the Nth item from the list.
func (ns *Dll[T]) Index(sub int) (rv *DllNode[T], err error) {
	(*ns).mu.RLock()
	defer (*ns).mu.Unlock()
	if ns.IsEmpty() {
		return nil, ErrOutOfRange 
	} 

	if sub < 0 || sub >= (*ns).length {
		return nil, ErrOutOfRange 
	} else if sub < ((*ns).length/2) {
		i := 0
		rv = (*ns).head;
		for ; i < sub; rv = rv.next {
			i++
		}
		return
	} else {
		i := (*ns).length-1
		rv = (*ns).tail;
		for ; rv != nil && i > sub; rv = rv.prev {
			i--
		}
		return
	}

	return nil, ErrOutOfRange 
}
