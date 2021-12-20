package dll

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.

Basic operations on a Doubly Linked List (SLL)
This list has head-and-tail pointers.

	IsEmpty() — Returns true if the dll is empty
	AppendSLL(t T) -
 	Length() int - 

*/

import (
	"errors"
)

// A node in the singly linked list
type DllNode[T any] struct {
	next, prev *DllNode[T]
	data *T
}
// Dll is a generic type buildt on top of a slice
type Dll[T any] struct {
	head, tail *DllNode[T]
	length int
}

// IsEmpty will return true if the stack is empty
func (ns *Dll[T]) IsEmpty() bool {
	// return (*ns).head == nil
	return (*ns).length == 0
}

// Push will append a new node to the end of the list.
func (ns *Dll[T]) InsertHeadSLL(t *T) {
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
func (ns *Dll[T]) AppendTailSLL(t *T) {
	x := DllNode[T] { data: t }	// Create the node
	if (*ns).head == nil {
		(*ns).head = &x
		(*ns).tail = &x
		(*ns).length = 1
	} else {
		(*ns).tail.next = &x
		x.prev = (*ns).tail
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


// Peek returns the top element of the stack or an error indicating that the stack is empty.
func (ns *Dll[T]) Peek() (rv *T, err error) {
	if ns.IsEmpty() {
		return nil, ErrEmptyDll
	} 
	rv = (*ns).head.data
	return 
}

